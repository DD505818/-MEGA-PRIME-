package main

// Tests for the durable audit halt (audit_halt.go / audit_clear.go):
// bounded retry, gap persistence across restarts, the operator clear
// path, and fail-closed behavior at every step.

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/omega-prime-delta/truthclient"
)

// flakyTruthCore fails vulture.fill appends a configurable number of
// times (then recovers), and/or fails specific event types outright.
type flakyTruthCore struct {
	mu            sync.Mutex
	events        []appendedEvent
	failNextFills int
	failTypes     map[string]bool
	fillAttempts  int
}

func (f *flakyTruthCore) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if body.EventType == truthclient.EventFill {
			f.fillAttempts++
			if f.failNextFills > 0 {
				f.failNextFills--
				http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
				return
			}
		}
		if f.failTypes[body.EventType] {
			http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
			return
		}
		var p map[string]interface{}
		_ = json.Unmarshal(body.Payload, &p)
		f.events = append(f.events, appendedEvent{EventType: body.EventType, Payload: p})
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	})
}

func (f *flakyTruthCore) run(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return srv.URL
}

func (f *flakyTruthCore) snapshot() []appendedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]appendedEvent, len(f.events))
	copy(out, f.events)
	return out
}

func eventIndex(events []appendedEvent, eventType string) int {
	for i, e := range events {
		if e.EventType == eventType {
			return i
		}
	}
	return -1
}

func fastRetry(t *testing.T) {
	t.Helper()
	old := fillAuditRetryDelay
	fillAuditRetryDelay = time.Millisecond
	t.Cleanup(func() { fillAuditRetryDelay = old })
}

func redisOn(t *testing.T, s *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: s.Addr()})
}

// engineOnMiniredis builds a fresh engine (a "restarted process") over an
// existing miniredis, sharing the AEGIS public key and truth client.
func engineOnMiniredis(s *miniredis.Miniredis, pub ed25519.PublicKey, tc *truthclient.Client) *ExecutionEngine {
	return &ExecutionEngine{
		redis:       redis.NewClient(&redis.Options{Addr: s.Addr()}),
		approvalPub: pub,
		orders:      make(map[string]*Order),
		paperMode:   true,
		truthClient: tc,
	}
}

// A fill audit that fails twice then succeeds is recovered by the bounded
// retry: no gap, no halt.
func TestFillAudit_RetryRecoversWithoutGap(t *testing.T) {
	fastRetry(t)
	f := &flakyTruthCore{failNextFills: 2}
	e, priv, s := testRig(t)
	e.truthClient = truthclient.New(f.run(t), "")

	e.processSignal(signedSignal(priv, time.Now()))

	if f.fillAttempts != 3 {
		t.Fatalf("fill attempts = %d, want 3 (2 failures + recovery)", f.fillAttempts)
	}
	for _, o := range e.orders {
		if o.State != StateFilled {
			t.Fatalf("order state = %s, want filled", o.State)
		}
		if o.Meta["audit_gap"] == true {
			t.Fatal("order marked audit_gap despite recovered fill audit")
		}
	}
	if e.auditHalted.Load() {
		t.Fatal("audit halt engaged despite recovered fill audit")
	}
	rdb := redisOn(t, s)
	if _, err := rdb.Get(context.Background(), auditHaltedKey).Result(); err != redis.Nil {
		t.Fatalf("halt key present after recovery: err=%v", err)
	}
	if got := eventTypesFrom(f.snapshot()); len(got) != 2 || got[0] != truthclient.EventOrderSubmitted || got[1] != truthclient.EventFill {
		t.Fatalf("events = %v, want [order_submitted fill]", got)
	}
}

func eventTypesFrom(events []appendedEvent) []string {
	out := []string{}
	for _, e := range events {
		out = append(out, e.EventType)
	}
	return out
}

// The full #7 lifecycle: fill audit fails permanently → durable gap +
// halt in Redis → restarted process still refuses → operator clear
// backfills the fill, audits the clear, and lifts the halt.
func TestFillAuditGap_DurableAcrossRestartAndOperatorClear(t *testing.T) {
	fastRetry(t)
	f := &flakyTruthCore{failTypes: map[string]bool{truthclient.EventFill: true}}
	e1, priv, s := testRig(t)
	tc := truthclient.New(f.run(t), "")
	e1.truthClient = tc

	e1.processSignal(signedSignal(priv, time.Now()))
	var gapOrder *Order
	for _, o := range e1.orders {
		gapOrder = o
	}
	if gapOrder.State != StateFilled || gapOrder.Meta["audit_gap"] != true {
		t.Fatalf("order = %+v, want filled + audit_gap", gapOrder)
	}
	if !e1.auditHalted.Load() {
		t.Fatal("auditHalted not set after permanent fill audit failure")
	}

	// Durable state must exist in Redis, readable by any process.
	rdb := redisOn(t, s)
	ctx := context.Background()
	if v, err := rdb.Get(ctx, auditHaltedKey).Result(); err != nil || v != "1" {
		t.Fatalf("durable halt key = %q, %v; want \"1\"", v, err)
	}
	gaps, err := redisGapRecords(ctx, rdb)
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := gaps[gapOrder.ID]
	if !ok {
		t.Fatalf("no durable gap record for order %s (have %v)", gapOrder.ID, gaps)
	}
	if rec.Fill["order_id"] != gapOrder.ID || rec.Attempts != fillAuditAttempts || rec.LastError == "" {
		t.Fatalf("gap record incomplete: %+v", rec)
	}

	// Restart: a fresh process over the same Redis restores the halt and
	// keeps refusing new submissions.
	e2 := engineOnMiniredis(s, e1.approvalPub, tc)
	e2.restoreAuditHaltState()
	if !e2.auditHalted.Load() {
		t.Fatal("halt not restored after restart")
	}
	e2.processSignal(signedSignalWithID(priv, time.Now(), "appr-test-2", "sig-test-2"))
	refused := false
	for _, o := range e2.orders {
		if o.Meta["cancel_reason"] == "AUDIT_HALTED" {
			refused = true
		}
	}
	if !refused {
		t.Fatal("restarted process submitted during durable audit halt")
	}

	// TruthCore recovers; operator clears. The gap must be BACKFILLED
	// (no acknowledgement needed) and the clear audited, in that order.
	f.mu.Lock()
	f.failTypes = map[string]bool{}
	f.mu.Unlock()
	sum, err := clearAuditHalt(ctx, rdb, tc, "op-test", nil)
	if err != nil {
		t.Fatalf("clearAuditHalt: %v", err)
	}
	if len(sum.Backfilled) != 1 || sum.Backfilled[0] != gapOrder.ID || len(sum.Acknowledged) != 0 {
		t.Fatalf("summary = %+v, want backfill of %s only", sum, gapOrder.ID)
	}
	events := f.snapshot()
	iFill := eventIndex(events, truthclient.EventFill)
	iCleared := eventIndex(events, truthclient.EventAuditHaltCleared)
	if iFill < 0 || iCleared < 0 || iFill > iCleared {
		t.Fatalf("event order wrong (fill=%d cleared=%d): %v", iFill, iCleared, eventTypesFrom(events))
	}
	if events[iFill].Payload["backfilled"] != true || events[iFill].Payload["order_id"] != gapOrder.ID {
		t.Fatalf("backfill payload wrong: %v", events[iFill].Payload)
	}
	if events[iCleared].Payload["operator"] != "op-test" {
		t.Fatalf("cleared event not bound to operator: %v", events[iCleared].Payload)
	}
	if _, err := rdb.Get(ctx, auditHaltedKey).Result(); err != redis.Nil {
		t.Fatalf("halt key still present after clear: %v", err)
	}
	if n, _ := rdb.HLen(ctx, auditGapsKey).Result(); n != 0 {
		t.Fatalf("gap records remain after clear: %d", n)
	}

	// The still-running (restarted) process observes the clear on its
	// next submission — no restart needed.
	e2.processSignal(signedSignalWithID(priv, time.Now(), "appr-test-3", "sig-test-3"))
	filled := false
	for _, o := range e2.orders {
		if o.SignalID == "sig-test-3" && o.State == StateFilled {
			filled = true
		}
	}
	if !filled {
		t.Fatal("submissions still refused after operator clear")
	}
}

// Seed one durable gap record + halt flag directly (as engageAuditHalt
// would have written them).
func seedGap(t *testing.T, rdb *redis.Client, orderID string) {
	t.Helper()
	rec := fillGapRecord{
		OrderID: orderID,
		Fill: map[string]interface{}{
			"order_id": orderID, "signal_id": "sig-x", "approval_id": "appr-x",
			"quantity": 0.01, "fill_price": 63000.0,
		},
		Attempts: fillAuditAttempts, LastError: "boom", RecordedAtMs: 1,
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := rdb.HSet(ctx, auditGapsKey, orderID, raw).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, auditHaltedKey, "1", 0).Err(); err != nil {
		t.Fatal(err)
	}
}

// A clear that cannot backfill and is not acknowledged must refuse and
// leave the halt fully in place (fail closed, nothing appended).
func TestAuditClear_UnacknowledgedGapRefusesToClear(t *testing.T) {
	f := &flakyTruthCore{failTypes: map[string]bool{truthclient.EventFill: true}}
	_, _, s := testRig(t)
	rdb := redisOn(t, s)
	seedGap(t, rdb, "ord-x")
	tc := truthclient.New(f.run(t), "")

	_, err := clearAuditHalt(context.Background(), rdb, tc, "op-test", nil)
	if err == nil || !strings.Contains(err.Error(), "not acknowledged") {
		t.Fatalf("err = %v, want unacknowledged-gap refusal", err)
	}
	ctx := context.Background()
	if v, _ := rdb.Get(ctx, auditHaltedKey).Result(); v != "1" {
		t.Fatal("halt lifted despite refused clear")
	}
	if eventIndex(f.snapshot(), truthclient.EventAuditHaltCleared) >= 0 {
		t.Fatal("audit_halt_cleared appended despite refused clear")
	}
}

// An acknowledged (unbackfillable) gap gets its explicit fill_audit_gap
// record appended BEFORE the halt_cleared record, and only then is the
// durable state deleted.
func TestAuditClear_AcknowledgedGapAppendsGapRecordThenClears(t *testing.T) {
	f := &flakyTruthCore{failTypes: map[string]bool{truthclient.EventFill: true}}
	_, _, s := testRig(t)
	rdb := redisOn(t, s)
	seedGap(t, rdb, "ord-x")
	tc := truthclient.New(f.run(t), "")

	sum, err := clearAuditHalt(context.Background(), rdb, tc, "op-test", []string{"ord-x"})
	if err != nil {
		t.Fatalf("clearAuditHalt: %v", err)
	}
	if len(sum.Acknowledged) != 1 || sum.Acknowledged[0] != "ord-x" || len(sum.Backfilled) != 0 {
		t.Fatalf("summary = %+v, want acknowledgement of ord-x only", sum)
	}
	events := f.snapshot()
	iGap := eventIndex(events, truthclient.EventFillAuditGap)
	iCleared := eventIndex(events, truthclient.EventAuditHaltCleared)
	if iGap < 0 || iCleared < 0 || iGap > iCleared {
		t.Fatalf("event order wrong (gap=%d cleared=%d): %v", iGap, iCleared, eventTypesFrom(events))
	}
	gp := events[iGap].Payload
	if gp["order_id"] != "ord-x" || gp["acknowledged_by"] != "op-test" || gp["gap_record_persisted"] != true {
		t.Fatalf("gap event payload wrong: %v", gp)
	}
	ctx := context.Background()
	if _, err := rdb.Get(ctx, auditHaltedKey).Result(); err != redis.Nil {
		t.Fatalf("halt key still present after acknowledged clear: %v", err)
	}
	if n, _ := rdb.HLen(ctx, auditGapsKey).Result(); n != 0 {
		t.Fatalf("gap records remain after clear: %d", n)
	}
}

// If the audit_halt_cleared append itself fails, NOTHING is cleared: the
// halt and the gap record stay durable (fail closed).
func TestAuditClear_ClearedEventAppendFailureKeepsHalt(t *testing.T) {
	f := &flakyTruthCore{failTypes: map[string]bool{
		truthclient.EventFill:             true, // backfill impossible
		truthclient.EventAuditHaltCleared: true, // and the clear cannot be audited
	}}
	_, _, s := testRig(t)
	rdb := redisOn(t, s)
	seedGap(t, rdb, "ord-x")
	tc := truthclient.New(f.run(t), "")

	_, err := clearAuditHalt(context.Background(), rdb, tc, "op-test", []string{"ord-x"})
	if err == nil || !strings.Contains(err.Error(), "halt NOT cleared") {
		t.Fatalf("err = %v, want audit-append failure", err)
	}
	ctx := context.Background()
	if v, _ := rdb.Get(ctx, auditHaltedKey).Result(); v != "1" {
		t.Fatal("halt lifted although the clear could not be audited")
	}
	if n, _ := rdb.HLen(ctx, auditGapsKey).Result(); n != 1 {
		t.Fatalf("gap record lost although clear failed: %d", n)
	}
}

// With no halt and no gaps, the clear is a no-op success.
func TestAuditClear_NothingToClear(t *testing.T) {
	f := &flakyTruthCore{}
	_, _, s := testRig(t)
	rdb := redisOn(t, s)
	tc := truthclient.New(f.run(t), "")

	sum, err := clearAuditHalt(context.Background(), rdb, tc, "op-test", nil)
	if err != nil {
		t.Fatalf("clearAuditHalt: %v", err)
	}
	if sum.Halted || len(sum.Backfilled) != 0 || len(sum.Acknowledged) != 0 {
		t.Fatalf("summary = %+v, want empty", sum)
	}
	if len(f.snapshot()) != 0 {
		t.Fatal("events appended for a no-op clear")
	}
}

// Boot with Redis unreachable: the halt state is UNKNOWN, so the process
// assumes HALTED (fail closed) rather than resuming blind.
func TestRestoreAuditHaltState_RedisDownFailsClosed(t *testing.T) {
	e, _ := deadRig(t)
	e.restoreAuditHaltState()
	if !e.auditHalted.Load() {
		t.Fatal("unreadable durable halt state did not fail closed at boot")
	}
}
