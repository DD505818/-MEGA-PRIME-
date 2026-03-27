package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

var (
	db *sql.DB

	mu    sync.RWMutex
	state struct{ equity, dailyLoss, peakEquity, drawdown float64 }

	leaderState atomic.Bool
	leaderMu    sync.Mutex
	leaderConn  *sql.Conn
	leaderID    string
)

const lockID = 12345

func initDB() error {
	var err error
	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	return db.Ping()
}

func tryBecomeLeader() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := db.Conn(ctx)
	if err != nil {
		return false
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil || !acquired {
		_ = conn.Close()
		return false
	}

	_, err = conn.ExecContext(ctx, `INSERT INTO leader_election (id, leader_id, last_heartbeat) VALUES (1, $1, NOW()) ON CONFLICT (id) DO UPDATE SET leader_id = EXCLUDED.leader_id, last_heartbeat = NOW()`, leaderID)
	if err != nil {
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID)
		_ = conn.Close()
		return false
	}

	leaderMu.Lock()
	if leaderConn != nil {
		_ = leaderConn.Close()
	}
	leaderConn = conn
	leaderMu.Unlock()

	leaderState.Store(true)
	return true
}

func releaseLeadership() {
	leaderMu.Lock()
	conn := leaderConn
	leaderConn = nil
	leaderMu.Unlock()

	if conn == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	_ = conn.Close()
}

func refreshHeartbeat() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for leaderState.Load() {
		<-ticker.C

		leaderMu.Lock()
		conn := leaderConn
		leaderMu.Unlock()
		if conn == nil {
			leaderState.Store(false)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := conn.ExecContext(ctx, "UPDATE leader_election SET last_heartbeat = NOW() WHERE id = 1 AND leader_id = $1", leaderID)
		cancel()
		if err != nil {
			leaderState.Store(false)
			releaseLeadership()
			return
		}
	}
}

func leaderElectionLoop() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()

	for range t.C {
		if leaderState.Load() {
			continue
		}

		var heartbeat time.Time
		var leader string
		err := db.QueryRow("SELECT leader_id, last_heartbeat FROM leader_election WHERE id = 1").Scan(&leader, &heartbeat)
		if err == nil && time.Since(heartbeat) < 15*time.Second {
			continue
		}

		if tryBecomeLeader() {
			go refreshHeartbeat()
		}
	}
}

func loadState() {
	var e, dl, p, d float64
	if err := db.QueryRow("SELECT equity, daily_loss, peak_equity, drawdown FROM account_state WHERE id = 1").Scan(&e, &dl, &p, &d); err == nil {
		mu.Lock()
		state = struct{ equity, dailyLoss, peakEquity, drawdown float64 }{e, dl, p, d}
		mu.Unlock()
	}
}

func refreshStateLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			loadState()
		case <-stop:
			return
		}
	}
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	if !leaderState.Load() {
		http.Error(w, "Not leader", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Order struct {
			Qty float64 `json:"qty"`
		} `json:"order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mu.RLock()
	eq, dl, dd := state.equity, state.dailyLoss, state.drawdown
	mu.RUnlock()

	if dl < -0.02*eq || dd > 0.10 || req.Order.Qty > 100 {
		http.Error(w, "Risk limits exceeded", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{"validated": true})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func main() {
	if err := initDB(); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	leaderID = os.Getenv("HOSTNAME")
	if leaderID == "" {
		leaderID = "risk-engine-" + time.Now().Format("20060102150405")
	}

	loadState()
	stopRefresh := make(chan struct{})
	go refreshStateLoop(stopRefresh)
	go leaderElectionLoop()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/validate", validateHandler)

	srv := &http.Server{Addr: ":3002"}
	go func() { _ = srv.ListenAndServe() }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	close(stopRefresh)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	if leaderState.Load() {
		leaderState.Store(false)
		releaseLeadership()
	}
}
