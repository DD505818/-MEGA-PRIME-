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
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

var (
	db *sql.DB

	leadershipMu sync.RWMutex
	isLeader     bool
	leaderID     string
	leaderConn   *sql.Conn
)

const lockID = 12345

type riskState struct {
	equity     float64
	dailyLoss  float64
	peakEquity float64
	drawdown   float64
}

func setLeadership(v bool) {
	leadershipMu.Lock()
	isLeader = v
	leadershipMu.Unlock()
}

func leader() bool {
	leadershipMu.RLock()
	defer leadershipMu.RUnlock()
	return isLeader
}

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
	err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil || !acquired {
		conn.Close()
		return false
	}
	_, err = conn.ExecContext(ctx, `INSERT INTO leader_election (id, leader_id, last_heartbeat)
                      VALUES (1, $1, NOW())
                      ON CONFLICT (id) DO UPDATE
                      SET leader_id = EXCLUDED.leader_id, last_heartbeat = NOW()`, leaderID)
	if err != nil {
		conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID)
		conn.Close()
		return false
	}
	leadershipMu.Lock()
	leaderConn = conn
	isLeader = true
	leadershipMu.Unlock()
	return true
}

func releaseLeadership() {
	leadershipMu.Lock()
	conn := leaderConn
	leaderConn = nil
	isLeader = false
	leadershipMu.Unlock()
	if conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID); err != nil {
		log.Printf("Failed to unlock advisory lock: %v", err)
	}
	if err := conn.Close(); err != nil {
		log.Printf("Failed to close leader connection: %v", err)
	}
}

func refreshHeartbeat() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !leader() {
			return
		}
		_, err := db.Exec("UPDATE leader_election SET last_heartbeat = NOW() WHERE id = 1 AND leader_id = $1", leaderID)
		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
			releaseLeadership()
			return
		}
	}
}

func leaderElectionLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if leader() {
			continue
		}
		var heartbeat time.Time
		var leader string
		err := db.QueryRow("SELECT leader_id, last_heartbeat FROM leader_election WHERE id = 1").Scan(&leader, &heartbeat)
		if err == nil && time.Since(heartbeat) < 15*time.Second {
			continue
		}
		if tryBecomeLeader() {
			log.Println("Became risk engine leader")
			go refreshHeartbeat()
		}
	}
}

func loadState(ctx context.Context) (riskState, error) {
	row := db.QueryRowContext(ctx, "SELECT equity, daily_loss, peak_equity, drawdown FROM account_state WHERE id = 1")
	var s riskState
	if err := row.Scan(&s.equity, &s.dailyLoss, &s.peakEquity, &s.drawdown); err != nil {
		return riskState{}, err
	}
	return s, nil
}

var loadRiskState = loadState

func validateHandler(w http.ResponseWriter, r *http.Request) {
	if !leader() {
		http.Error(w, "Not leader", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	state, err := loadRiskState(ctx)
	if err != nil {
		log.Printf("Failed to load state for validation: %v", err)
		http.Error(w, "Risk state unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Order struct {
			OrderID string  `json:"orderId"`
			Symbol  string  `json:"symbol"`
			Side    string  `json:"side"`
			Qty     float64 `json:"qty"`
			Type    string  `json:"type"`
			Price   float64 `json:"price"`
			Agent   string  `json:"agent"`
		} `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if state.dailyLoss < -0.02*state.equity {
		http.Error(w, "Daily loss limit exceeded", http.StatusBadRequest)
		return
	}
	if state.drawdown > 0.10 {
		http.Error(w, "Drawdown limit exceeded", http.StatusBadRequest)
		return
	}
	if req.Order.Qty > 100 {
		http.Error(w, "Position size limit exceeded", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"validated": true})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	if err := initDB(); err != nil {
		log.Fatal("DB init failed:", err)
	}
	defer db.Close()

	leaderID = os.Getenv("HOSTNAME")
	if leaderID == "" {
		leaderID = "risk-engine-" + time.Now().Format("20060102150405")
	}
	go leaderElectionLoop()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/validate", validateHandler)

	srv := &http.Server{Addr: ":3002"}
	go func() {
		log.Println("Risk engine listening on :3002")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	if leader() {
		releaseLeadership()
	}
}
