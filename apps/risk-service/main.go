package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/omega-prime-delta/approval"
	"github.com/omega-prime-delta/modelock"
	"github.com/omega-prime-delta/truthclient"
)

var engine *RiskEngine

func liveHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"live"}`))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := engine.redis.Ping(ctx).Err(); err != nil {
		http.Error(w, `{"status":"not_ready","dependency":"redis"}`, http.StatusServiceUnavailable)
		return
	}
	if _, err := engine.producer.GetMetadata(nil, true, 2000); err != nil {
		http.Error(w, `{"status":"not_ready","dependency":"kafka"}`, http.StatusServiceUnavailable)
		return
	}
	if engine.truthClient == nil || engine.truthClient.Ready(ctx) != nil {
		http.Error(w, `{"status":"not_ready","dependency":"truth_core"}`, http.StatusServiceUnavailable)
		return
	}
	assignment, err := engine.consumer.Assignment()
	if err != nil || len(assignment) == 0 {
		http.Error(w, `{"status":"not_ready","dependency":"kafka_consumer"}`, http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte(`{"status":"ready"}`))
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
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "manual"
	}
	actor := actorFromContext(r)
	engine.recordControlAudit("kill", actor, true, "attempt", req.Reason)
	already, err := engine.activateKillSwitch(req.Reason)
	if err != nil {
		// Durable write failed: the switch is NOT active. Report 503 rather
		// than a success that would be false confidence.
		engine.recordControlAudit("kill", actor, true, "failed", req.Reason+": "+err.Error())
		http.Error(w, `{"error":"kill failed: durable state write failed"}`, http.StatusServiceUnavailable)
		return
	}
	if already {
		engine.recordControlAudit("kill", actor, true, "already_active", req.Reason)
	} else {
		engine.recordControlAudit("kill", actor, true, "activated", req.Reason)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "kill_switch_activated",
		"already_active": already,
	})
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r)
	engine.recordControlAudit("reset", actor, true, "attempt", "operator reset")
	if err := engine.resetKillSwitch(); err != nil {
		engine.recordControlAudit("reset", actor, true, "failed", err.Error())
		http.Error(w, `{"error":"reset failed: durable state write failed"}`, http.StatusServiceUnavailable)
		return
	}
	engine.recordControlAudit("reset", actor, true, "completed", "kill switch and circuit breaker cleared")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"killed":         engine.killSwitch.Load(),
		"circuit":        engine.circuitBreak.Load(),
		"state_verified": engine.stateVerified.Load(),
		"control_seq":    engine.controlSeq.Load(),
	})
}

// restoreControlState re-arms the in-memory kill switch from the durable
// Redis flag on boot, so a kill survives process restarts. A kill is cleared
// only via the authenticated /reset endpoint — never by restarting.
//
// Fail-closed: if Redis is unreachable at boot, the switch is assumed KILLED
// and stateVerified stays false. A background retry re-reads the flag until
// it succeeds; only then does the in-memory state trust the durable value.
// A risk-service that booted armed because Redis was down would be worse
// than one that boots dead.
func (e *RiskEngine) restoreControlState() {
	if e.tryRestoreControlState() {
		return
	}
	e.killSwitch.Store(true)
	e.stateVerified.Store(false)
	log.Println("CONTROL: Redis unavailable at boot — assuming KILLED (fail-closed); retrying in background")
	go e.retryControlStateRestore()
}

// tryRestoreControlState reads the durable flag once. Returns true when the
// read succeeded (state is now verified), false when Redis was unreachable.
func (e *RiskEngine) tryRestoreControlState() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	val, err := e.redis.Get(ctx, "kill_switch").Result()
	if err != nil && err != redis.Nil {
		return false
	}
	e.killSwitch.Store(val == "1")
	e.stateVerified.Store(true)
	if val == "1" {
		log.Println("CONTROL: kill switch restored from durable state (kill_switch=1)")
	} else {
		log.Println("CONTROL: no durable kill flag — starting disarmed (state verified)")
	}
	return true
}

// retryControlStateRestore re-attempts the durable read until it succeeds.
func (e *RiskEngine) retryControlStateRestore() {
	t := time.NewTicker(controlRestoreRetryInterval)
	defer t.Stop()
	for range t.C {
		if e.tryRestoreControlState() {
			log.Println("CONTROL: durable kill state verified after retry")
			return
		}
	}
}

// controlRestoreRetryInterval is a var so tests can shorten the retry loop.
var controlRestoreRetryInterval = 10 * time.Second

func main() {
	// PAPER/LIVE lock: fail closed unless explicitly in paper mode.
	// LIVE is locked — no configuration can enable it.
	modelock.RequirePaper("risk-service")

	// Fail closed: the control plane must not start without operator auth.
	secret, err := resolveOperatorSecret()
	if err != nil {
		log.Fatalf("control-plane auth: %v", err)
	}

	// Authority boundary: AEGIS signs every approval with this key. Without
	// a valid signing key no approval can be issued, so the service must not
	// start — otherwise validate() would forward unsigned signals.
	approvalPriv, err := approval.ParsePrivateKey(os.Getenv("AEGIS_APPROVAL_PRIVKEY"))
	if err != nil {
		log.Fatalf("authority boundary: %v", err)
	}

	engine = NewRiskEngine(
		os.Getenv("REDIS_URL"),
		os.Getenv("KAFKA_BROKERS"),
	)
	engine.approvalPriv = approvalPriv
	// TruthCore audit client (Phase 4, audit-or-no-trade). Writes are
	// authenticated when TRUTHCORE_WRITE_SECRET is set.
	truthURL := os.Getenv("TRUTHCORE_URL")
	if truthURL == "" {
		truthURL = "http://truth-core:8084"
	}
	engine.truthClient = truthclient.New(truthURL, os.Getenv("TRUTHCORE_WRITE_SECRET"))
	engine.restoreControlState()
	go engine.run()
	go engine.reconcilePositionsLoop() // 1B.2: 60s position reconcile, fail-closed on divergence
	go engine.truthVerifyLoop()        // Phase 4: 60s independent chain verification, kill on tamper

	mux := http.NewServeMux()
	mux.HandleFunc("/health", liveHandler)
	mux.HandleFunc("/health/live", liveHandler)
	mux.HandleFunc("/health/ready", readyHandler)
	mux.HandleFunc("/validate", validateHandler)
	// Control plane: authenticated. /reset requires admin — strictly stronger
	// than /kill (operator), because reset re-arms the system after a kill.
	mux.Handle("/kill", requireControlAuth(engine, secret, roleOperator, "kill", http.HandlerFunc(killHandler)))
	mux.Handle("/reset", requireControlAuth(engine, secret, roleAdmin, "reset", http.HandlerFunc(resetHandler)))
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
