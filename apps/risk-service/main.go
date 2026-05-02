package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/google/uuid"
)

var engine *RiskEngine

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte(`{"status":"ok"}`))
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	var sig map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&sig); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	ok, reason, qty := engine.validate(sig)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approved": ok, "reason": reason, "quantity": qty,
		"trace_id": uuid.NewString(),
	})
}

func killHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Reason string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "manual"
	}
	engine.activateKillSwitch(req.Reason)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "kill_switch_activated"})
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	engine.killSwitch.Store(false)
	engine.circuitBreak.Store(false)
	ctx := context.Background()
	engine.redis.Del(ctx, "kill_switch")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"killed":  engine.killSwitch.Load(),
		"circuit": engine.circuitBreak.Load(),
	})
}

func main() {
	engine = NewRiskEngine(
		os.Getenv("REDIS_URL"),
		os.Getenv("KAFKA_BROKERS"),
	)
	go engine.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/validate", validateHandler)
	mux.HandleFunc("/kill", killHandler)
	mux.HandleFunc("/reset", resetHandler)
	mux.HandleFunc("/status", statusHandler)

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		log.Println("Risk service HTTP on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down risk service")
}
