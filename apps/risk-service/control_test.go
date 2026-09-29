package main

// End-to-end control-plane tests: kill/reset authorization, kill-state
// persistence across restarts, and durable audit events.
// Run: go test -run 'TestKill|TestReset|TestControlAudit' -v

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// errFakeRedisDown simulates Redis being unreachable.
var errFakeRedisDown = errors.New("fake redis unavailable")

func jsonDecode(rr *httptest.ResponseRecorder, v interface{}) error {
	return json.NewDecoder(rr.Body).Decode(v)
}

// controlMux wires /kill and /reset exactly like main(), against a test engine.
func controlMux(t *testing.T, eng *RiskEngine) (*http.ServeMux, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)
	t.Setenv("TRUTHCORE_URL", "http://127.0.0.1:1") // fail fast, no network
	secret, err := resolveOperatorSecret()
	if err != nil {
		t.Fatal(err)
	}
	engine = eng // handlers use the package-level engine
	mux := http.NewServeMux()
	mux.Handle("/kill", requireControlAuth(engine, secret, roleOperator, "kill", http.HandlerFunc(killHandler)))
	mux.Handle("/reset", requireControlAuth(engine, secret, roleAdmin, "reset", http.HandlerFunc(resetHandler)))
	return mux, secret
}

func doPost(t *testing.T, mux *http.ServeMux, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func mint(t *testing.T, secret, sub, role string, ttl time.Duration) string {
	t.Helper()
	tok, err := mintOperatorToken(secret, sub, role, []string{"risk-service"}, ttl)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestKillUnauthorized(t *testing.T) {
	fr := newFakeRedis()
	eng := &RiskEngine{redis: fr}
	mux, _ := controlMux(t, eng)

	rr := doPost(t, mux, "/kill", "", `{"reason":"attack"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized kill: got %d, want 401", rr.Code)
	}
	if eng.killSwitch.Load() {
		t.Fatal("kill switch must not activate on unauthorized request")
	}
	if _, err := fr.Get(context.Background(), "kill_switch").Result(); err == nil {
		t.Fatal("kill_switch key must not be set on unauthorized request")
	}
	entries := fr.streamEntries(controlAuditStream)
	if len(entries) == 0 {
		t.Fatal("denied kill attempt must be audit-logged")
	}
	if entries[0]["authorized"] != "false" || entries[0]["action"] != "kill" {
		t.Fatalf("bad audit entry: %v", entries[0])
	}
}

func TestKillAuthorized(t *testing.T) {
	fr := newFakeRedis()
	fr.Set(context.Background(), "kill:confirmed", "1", 0) // silence cascade goroutine
	eng := &RiskEngine{redis: fr}
	mux, secret := controlMux(t, eng)

	tok := mint(t, secret, "op1", "operator", time.Hour)
	rr := doPost(t, mux, "/kill", tok, `{"reason":"drill"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("authorized kill: got %d, want 200 (body %s)", rr.Code, rr.Body.String())
	}
	if !eng.killSwitch.Load() {
		t.Fatal("kill switch must be active after authorized kill")
	}
	v, err := fr.Get(context.Background(), "kill_switch").Result()
	if err != nil || v != "1" {
		t.Fatalf("durable kill_switch flag missing: v=%q err=%v", v, err)
	}
	var granted bool
	for _, e := range fr.streamEntries(controlAuditStream) {
		if e["action"] == "kill" && e["authorized"] == "true" && e["outcome"] == "activated" {
			granted = true
		}
	}
	if !granted {
		t.Fatal("authorized kill must produce an audit entry with outcome=activated")
	}
}

func TestResetRequiresAdmin(t *testing.T) {
	fr := newFakeRedis()
	eng := &RiskEngine{redis: fr}
	eng.killSwitch.Store(true)
	fr.Set(context.Background(), "kill_switch", "1", 0)
	mux, secret := controlMux(t, eng)

	// Operator (non-admin) must be forbidden.
	opTok := mint(t, secret, "op1", "operator", time.Hour)
	rr := doPost(t, mux, "/reset", opTok, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("operator reset: got %d, want 403", rr.Code)
	}
	if !eng.killSwitch.Load() {
		t.Fatal("kill switch must stay active after forbidden reset")
	}

	// Admin reset succeeds and clears durable state.
	adminTok := mint(t, secret, "admin1", "admin", time.Hour)
	rr = doPost(t, mux, "/reset", adminTok, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("admin reset: got %d, want 200 (body %s)", rr.Code, rr.Body.String())
	}
	if eng.killSwitch.Load() {
		t.Fatal("kill switch must be cleared after admin reset")
	}
	if _, err := fr.Get(context.Background(), "kill_switch").Result(); err == nil {
		t.Fatal("durable kill_switch flag must be deleted after reset")
	}
	var resetDone bool
	for _, e := range fr.streamEntries(controlAuditStream) {
		if e["action"] == "reset" && e["outcome"] == "completed" && e["actor"] == "admin1" {
			resetDone = true
		}
	}
	if !resetDone {
		t.Fatal("admin reset must be audit-logged with outcome=completed")
	}
}

func TestResetUnauthorized(t *testing.T) {
	fr := newFakeRedis()
	eng := &RiskEngine{redis: fr}
	mux, _ := controlMux(t, eng)

	rr := doPost(t, mux, "/reset", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized reset: got %d, want 401", rr.Code)
	}
}

func TestKillStatePersistenceAcrossRestart(t *testing.T) {
	fr := newFakeRedis()
	fr.Set(context.Background(), "kill:confirmed", "1", 0)
	eng1 := &RiskEngine{redis: fr}
	mux, secret := controlMux(t, eng1)

	tok := mint(t, secret, "op1", "operator", time.Hour)
	if rr := doPost(t, mux, "/kill", tok, `{"reason":"persistence-check"}`); rr.Code != http.StatusOK {
		t.Fatalf("kill: got %d", rr.Code)
	}

	// Simulate a process restart: brand-new engine, same durable store.
	eng2 := &RiskEngine{redis: fr}
	eng2.restoreControlState()
	if !eng2.killSwitch.Load() {
		t.Fatal("kill switch must be restored from durable state after restart")
	}

	// And a clean store means a clean boot.
	eng3 := &RiskEngine{redis: newFakeRedis()}
	eng3.restoreControlState()
	if eng3.killSwitch.Load() {
		t.Fatal("kill switch must not be active on clean boot")
	}
}

func TestControlAuditDeniedAttemptLogged(t *testing.T) {
	fr := newFakeRedis()
	eng := &RiskEngine{redis: fr}
	mux, _ := controlMux(t, eng)

	doPost(t, mux, "/reset", "", "")
	entries := fr.streamEntries(controlAuditStream)
	if len(entries) == 0 {
		t.Fatal("denied reset attempt must be audit-logged")
	}
	e := entries[0]
	if e["action"] != "reset" || e["authorized"] != "false" || e["outcome"] != "denied" {
		t.Fatalf("bad audit entry: %v", e)
	}
}

// ── Phase 1A review: blocking items ───────────────────────────────────────────

// TestKillIdempotent: killing twice returns 200 both times; the second is
// reported as already_active and audited distinctly.
func TestKillIdempotent(t *testing.T) {
	fr := newFakeRedis()
	fr.Set(context.Background(), "kill:confirmed", "1", 0)
	eng := &RiskEngine{redis: fr}
	mux, secret := controlMux(t, eng)
	tok := mint(t, secret, "op1", "operator", time.Hour)

	rr1 := doPost(t, mux, "/kill", tok, `{"reason":"first"}`)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first kill: got %d", rr1.Code)
	}
	var b1 map[string]interface{}
	if err := jsonDecode(rr1, &b1); err != nil || b1["already_active"] != false {
		t.Fatalf("first kill must report already_active=false: %v %s", err, rr1.Body.String())
	}

	rr2 := doPost(t, mux, "/kill", tok, `{"reason":"second"}`)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second kill: got %d, want 200 (idempotent)", rr2.Code)
	}
	var b2 map[string]interface{}
	if err := jsonDecode(rr2, &b2); err != nil || b2["already_active"] != true {
		t.Fatalf("second kill must report already_active=true: %v %s", err, rr2.Body.String())
	}

	var alreadyLogged bool
	for _, e := range fr.streamEntries(controlAuditStream) {
		if e["action"] == "kill" && e["outcome"] == "already_active" {
			alreadyLogged = true
		}
	}
	if !alreadyLogged {
		t.Fatal("idempotent re-kill must be audited with outcome=already_active")
	}
}

// TestKillFailsClosedWhenRedisDown: if the durable write fails, the endpoint
// returns 503 and the in-memory flag is NEVER set — there is no code path
// where memory says "killed" but Redis does not. This is the unit-testable
// half of the kill -9 mid-/kill analysis: the commit order is durable-first,
// so a crash between the Redis write and the memory store can only leave the
// switch durably KILLED, which restoreControlState picks up at next boot.
func TestKillFailsClosedWhenRedisDown(t *testing.T) {
	fr := newFakeRedis()
	fr.setFailSet(errFakeRedisDown)
	eng := &RiskEngine{redis: fr}
	mux, secret := controlMux(t, eng)
	tok := mint(t, secret, "op1", "operator", time.Hour)

	rr := doPost(t, mux, "/kill", tok, `{"reason":"redis-down"}`)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("kill with redis down: got %d, want 503", rr.Code)
	}
	if eng.killSwitch.Load() {
		t.Fatal("in-memory kill flag must NOT be set when the durable write failed")
	}
	var failedLogged bool
	for _, e := range fr.streamEntries(controlAuditStream) {
		if e["action"] == "kill" && e["outcome"] == "failed" {
			failedLogged = true
		}
	}
	if !failedLogged {
		t.Fatal("failed kill must be audited with outcome=failed")
	}
}

// TestResetFailsClosedWhenRedisDown: a failed durable delete rolls the
// in-memory flags back and returns 503 — no phantom reset.
func TestResetFailsClosedWhenRedisDown(t *testing.T) {
	fr := newFakeRedis()
	fr.Set(context.Background(), "kill:confirmed", "1", 0)
	eng := &RiskEngine{redis: fr}
	mux, secret := controlMux(t, eng)
	opTok := mint(t, secret, "op1", "operator", time.Hour)
	adminTok := mint(t, secret, "admin1", "admin", time.Hour)

	if rr := doPost(t, mux, "/kill", opTok, ""); rr.Code != http.StatusOK {
		t.Fatalf("setup kill: got %d", rr.Code)
	}
	fr.setFailDel(errFakeRedisDown)
	rr := doPost(t, mux, "/reset", adminTok, "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("reset with redis down: got %d, want 503", rr.Code)
	}
	if !eng.killSwitch.Load() {
		t.Fatal("kill flag must roll back to active when the durable delete failed")
	}
	if v, _ := fr.Get(context.Background(), "kill_switch").Result(); v != "1" {
		t.Fatal("durable kill flag must still be present after failed reset")
	}
}

// TestConcurrentKillResetNoDivergence: hammer kill and reset from many
// goroutines; the mutex + monotonic seq must keep the in-memory flag and the
// durable flag in agreement, and the in-memory seq must match Redis.
func TestConcurrentKillResetNoDivergence(t *testing.T) {
	fr := newFakeRedis()
	eng := &RiskEngine{redis: fr}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_, _ = eng.activateKillSwitch("storm")
			} else {
				_ = eng.resetKillSwitch()
			}
		}(i)
	}
	wg.Wait()

	durable, derr := fr.Get(context.Background(), "kill_switch").Result()
	if derr != nil && derr != redis.Nil {
		t.Fatalf("unexpected redis error: %v", derr)
	}
	if eng.killSwitch.Load() != (durable == "1") {
		t.Fatalf("DIVERGENCE: in-memory=%t durable=%q", eng.killSwitch.Load(), durable)
	}
	seqStr, _ := fr.Get(context.Background(), controlSeqKey).Result()
	var seq int64
	_, _ = fmt.Sscanf(seqStr, "%d", &seq)
	if eng.controlSeq.Load() != seq {
		t.Fatalf("seq divergence: in-memory=%d redis=%d", eng.controlSeq.Load(), seq)
	}
	if seq == 0 {
		t.Fatal("expected control:seq to advance under concurrent transitions")
	}
}

// TestRestoreControlStateFailClosed: Redis down at boot -> assume KILLED,
// state unverified; background retry verifies once Redis recovers.
func TestRestoreControlStateFailClosed(t *testing.T) {
	old := controlRestoreRetryInterval
	controlRestoreRetryInterval = 50 * time.Millisecond
	defer func() { controlRestoreRetryInterval = old }()

	fr := newFakeRedis()
	fr.setFailGet(errFakeRedisDown)
	eng := &RiskEngine{redis: fr}
	eng.restoreControlState()
	if !eng.killSwitch.Load() {
		t.Fatal("boot with redis down must assume KILLED (fail closed)")
	}
	if eng.stateVerified.Load() {
		t.Fatal("state must be unverified while redis is unreachable")
	}

	// Redis recovers with no durable kill flag: the retry loop must verify
	// and disarm.
	fr.setFailGet(nil)
	deadline := time.Now().Add(3 * time.Second)
	for !eng.stateVerified.Load() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !eng.stateVerified.Load() {
		t.Fatal("background retry never verified state after redis recovered")
	}
	if eng.killSwitch.Load() {
		t.Fatal("no durable flag means the switch must disarm once verified")
	}

	// And a durable "1" discovered on retry must re-arm.
	fr.Set(context.Background(), "kill_switch", "1", 0)
	eng2 := &RiskEngine{redis: fr}
	eng2.restoreControlState()
	if !eng2.killSwitch.Load() || !eng2.stateVerified.Load() {
		t.Fatal("durable kill flag must restore on boot when redis is reachable")
	}
}

// TestAuditGapRecordedWhenTruthCoreDown: every audit event carries an
// audit_id; when the TruthCore append fails, a gap record with the same
// audit_id, failed_at, and reason lands in control:audit:gaps.
func TestAuditGapRecordedWhenTruthCoreDown(t *testing.T) {
	fr := newFakeRedis()
	fr.Set(context.Background(), "kill:confirmed", "1", 0)
	eng := &RiskEngine{redis: fr}
	mux, secret := controlMux(t, eng) // TRUTHCORE_URL points at a dead port
	tok := mint(t, secret, "op1", "operator", time.Hour)

	if rr := doPost(t, mux, "/kill", tok, `{"reason":"gap-check"}`); rr.Code != http.StatusOK {
		t.Fatalf("kill: got %d", rr.Code)
	}
	gaps := fr.streamEntries(controlAuditGapStream)
	if len(gaps) == 0 {
		t.Fatal("truth-core append failure must produce gap records")
	}
	audits := fr.streamEntries(controlAuditStream)
	auditIDs := map[string]bool{}
	for _, e := range audits {
		if id, ok := e["audit_id"].(string); ok && id != "" {
			auditIDs[id] = true
		}
	}
	for _, g := range gaps {
		id, _ := g["audit_id"].(string)
		if id == "" || !auditIDs[id] {
			t.Fatalf("gap record has uncorrelatable audit_id %q", id)
		}
		if g["failed_at"] == nil || g["failed_at"] == "" {
			t.Fatalf("gap record missing failed_at: %v", g)
		}
		if g["reason"] == nil || g["reason"] == "" {
			t.Fatalf("gap record missing reason: %v", g)
		}
	}
}
