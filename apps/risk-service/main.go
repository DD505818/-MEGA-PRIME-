package main

import (
    "context"
    "encoding/json"
    "net/http"
    "os"
    "os/signal"
)

var engine *RiskEngine

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(200)
}
func validateHandler(w http.ResponseWriter, r *http.Request) {
    var sig map[string]interface{}
    json.NewDecoder(r.Body).Decode(&sig)
    ok, reason, qty := engine.validate(sig)
    json.NewEncoder(w).Encode(map[string]interface{}{"approved": ok, "reason": reason, "quantity": qty})
}
func killHandler(w http.ResponseWriter, r *http.Request) {
    var req struct{ Reason string }
    json.NewDecoder(r.Body).Decode(&req)
    if req.Reason == "" {
        req.Reason = "manual"
    }
    engine.activateKillSwitch(req.Reason)
    w.WriteHeader(200)
}
func resetHandler(w http.ResponseWriter, r *http.Request) {
    engine.killSwitch.Store(false)
    engine.redis.Del(context.Background(), "circuit_breaker:tripped")
    w.WriteHeader(200)
}
func statusHandler(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]interface{}{
        "killed":  engine.killSwitch.Load(),
        "circuit": engine.circuitBreak.Load(),
    })
}

func main() {
    engine = NewRiskEngine(os.Getenv("REDIS_URL"), os.Getenv("KAFKA_BROKERS"))
    go engine.run()
    http.HandleFunc("/health", healthHandler)
    http.HandleFunc("/validate", validateHandler)
    http.HandleFunc("/kill", killHandler)
    http.HandleFunc("/reset", resetHandler)
    http.HandleFunc("/status", statusHandler)
    go http.ListenAndServe(":8080", nil)
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt)
    <-sig
}
