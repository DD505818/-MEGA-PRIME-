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
    db       *sql.DB
    mu       sync.RWMutex
    state    struct { equity, dailyLoss, peakEquity, drawdown float64 }
    isLeader bool
    leaderID string
)

const lockID = 12345

func initDB() error { var err error; db, err = sql.Open("postgres", os.Getenv("DATABASE_URL")); if err != nil { return err }; return db.Ping() }
func tryBecomeLeader() bool {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second); defer cancel()
    var acquired bool
    if err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil || !acquired { return false }
    _, err := db.Exec(`INSERT INTO leader_election (id, leader_id, last_heartbeat) VALUES (1, $1, NOW()) ON CONFLICT (id) DO UPDATE SET leader_id = EXCLUDED.leader_id, last_heartbeat = NOW()`, leaderID)
    return err == nil
}
func releaseLeadership() { _, _ = db.Exec("SELECT pg_advisory_unlock($1)", lockID) }
func refreshHeartbeat() { for isLeader { time.Sleep(5 * time.Second); if _, err := db.Exec("UPDATE leader_election SET last_heartbeat = NOW() WHERE id = 1 AND leader_id = $1", leaderID); err != nil { isLeader = false; releaseLeadership(); return } } }
func leaderElectionLoop() {
    t := time.NewTicker(10 * time.Second)
    for range t.C {
        if isLeader { continue }
        var heartbeat time.Time; var leader string
        err := db.QueryRow("SELECT leader_id, last_heartbeat FROM leader_election WHERE id = 1").Scan(&leader, &heartbeat)
        if err == nil && time.Since(heartbeat) < 15*time.Second { continue }
        if tryBecomeLeader() { isLeader = true; go refreshHeartbeat() }
    }
}
func loadState() { var e, dl, p, d float64; if err := db.QueryRow("SELECT equity, daily_loss, peak_equity, drawdown FROM account_state WHERE id = 1").Scan(&e, &dl, &p, &d); err == nil { mu.Lock(); state = struct{ equity, dailyLoss, peakEquity, drawdown float64 }{e, dl, p, d}; mu.Unlock() } }
func validateHandler(w http.ResponseWriter, r *http.Request) {
    if !isLeader { http.Error(w, "Not leader", http.StatusServiceUnavailable); return }
    var req struct { Order struct { Qty float64 `json:"qty"` } `json:"order"` }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "Invalid request", http.StatusBadRequest); return }
    mu.RLock(); eq, dl, dd := state.equity, state.dailyLoss, state.drawdown; mu.RUnlock()
    if dl < -0.02*eq || dd > 0.10 || req.Order.Qty > 100 { http.Error(w, "Risk limits exceeded", http.StatusBadRequest); return }
    w.WriteHeader(http.StatusOK); _ = json.NewEncoder(w).Encode(map[string]bool{"validated": true})
}
func healthHandler(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("OK")) }
func main() {
    if err := initDB(); err != nil { log.Fatal(err) }
    defer db.Close()
    leaderID = os.Getenv("HOSTNAME"); if leaderID == "" { leaderID = "risk-engine-" + time.Now().Format("20060102150405") }
    loadState(); go leaderElectionLoop()
    http.HandleFunc("/health", healthHandler); http.HandleFunc("/validate", validateHandler)
    srv := &http.Server{Addr: ":3002"}
    go func() { _ = srv.ListenAndServe() }()
    quit := make(chan os.Signal, 1); signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM); <-quit
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel(); _ = srv.Shutdown(ctx)
    if isLeader { releaseLeadership() }
}
