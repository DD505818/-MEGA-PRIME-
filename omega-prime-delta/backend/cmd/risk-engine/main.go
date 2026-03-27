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
	db    *sql.DB
	mu    sync.RWMutex
	state struct {
		equity     float64
		dailyLoss  float64
		peakEquity float64
		drawdown   float64
	}
	isLeader bool
	leaderID string
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
	var acquired bool
	err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil || !acquired {
		return false
	}
	_, err = db.Exec(`INSERT INTO leader_election (id, leader_id, last_heartbeat)
                      VALUES (1, $1, NOW())
                      ON CONFLICT (id) DO UPDATE
                      SET leader_id = EXCLUDED.leader_id, last_heartbeat = NOW()`, leaderID)
	if err != nil {
		return false
	}
	return true
}

func releaseLeadership() {
	db.Exec("SELECT pg_advisory_unlock($1)", lockID)
}

func refreshHeartbeat() {
	for isLeader {
		time.Sleep(5 * time.Second)
		_, err := db.Exec("UPDATE leader_election SET last_heartbeat = NOW() WHERE id = 1 AND leader_id = $1", leaderID)
		if err != nil {
			log.Printf("Heartbeat failed: %v", err)
			isLeader = false
			releaseLeadership()
			return
		}
	}
}

func leaderElectionLoop() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		if isLeader {
			continue
		}
		var heartbeat time.Time
		var leader string
		err := db.QueryRow("SELECT leader_id, last_heartbeat FROM leader_election WHERE id = 1").Scan(&leader, &heartbeat)
		if err == nil && time.Since(heartbeat) < 15*time.Second {
			continue
		}
		if tryBecomeLeader() {
			isLeader = true
			log.Println("Became risk engine leader")
			go refreshHeartbeat()
		}
	}
}

func loadState() {
	row := db.QueryRow("SELECT equity, daily_loss, peak_equity, drawdown FROM account_state WHERE id = 1")
	var equity, dailyLoss, peakEquity, drawdown float64
	if err := row.Scan(&equity, &dailyLoss, &peakEquity, &drawdown); err != nil {
		log.Printf("Failed to load state: %v", err)
		return
	}
	mu.Lock()
	state.equity = equity
	state.dailyLoss = dailyLoss
	state.peakEquity = peakEquity
	state.drawdown = drawdown
	mu.Unlock()
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	if !isLeader {
		http.Error(w, "Not leader", http.StatusServiceUnavailable)
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
	mu.RLock()
	equity := state.equity
	dailyLoss := state.dailyLoss
	drawdown := state.drawdown
	mu.RUnlock()

	if dailyLoss < -0.02*equity {
		http.Error(w, "Daily loss limit exceeded", http.StatusBadRequest)
		return
	}
	if drawdown > 0.10 {
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
	loadState()
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
	if isLeader {
		releaseLeadership()
	}
}
