package main

// Phase 1B.2 — position-count reconciliation.
//
// Failure mode under test: the position state execution-service maintains
// diverges from the fills ledger (corrupt writer, partial write, external
// tampering, or a set/count inconsistency), and the reconciler must fail
// closed — audit + alert + kill — instead of letting Gate 8 decide on
// untrusted state.
//
// Run: go test -run 'TestReconcile|TestGate8' -v

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func seedFill(t *testing.T, fr *fakeRedis, symbol, side string, qty float64) {
	t.Helper()
	ctx := context.Background()
	rec := fmt.Sprintf(`{"order_id":"o-%s","symbol":%q,"side":%q,"qty":%v}`, symbol, symbol, side, qty)
	if err := fr.LPush(ctx, "portfolio:fills:"+symbol, rec).Err(); err != nil {
		t.Fatal(err)
	}
	if err := fr.SAdd(ctx, "portfolio:tracked_symbols", symbol).Err(); err != nil {
		t.Fatal(err)
	}
}

func seedPosition(t *testing.T, fr *fakeRedis, symbol string, qty float64, inOpenSet bool) {
	t.Helper()
	ctx := context.Background()
	if err := fr.Set(ctx, "portfolio:position:"+symbol, fmt.Sprintf("%v", qty), 0).Err(); err != nil {
		t.Fatal(err)
	}
	if inOpenSet {
		if err := fr.SAdd(ctx, "portfolio:open_symbols", symbol).Err(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReconcileHealthyNoKill(t *testing.T) {
	eng, fr := testRiskEngine(t)
	seedFill(t, fr, "BTCUSDT", "BUY", 1.0)
	seedFill(t, fr, "BTCUSDT", "SELL", 0.4)
	seedPosition(t, fr, "BTCUSDT", 0.6, true)

	if divs := eng.reconcilePositions(); len(divs) != 0 {
		t.Fatalf("healthy state must not diverge, got %v", divs)
	}
	if eng.killSwitch.Load() {
		t.Fatal("healthy reconcile must not trip the kill switch")
	}
}

func TestReconcileEmptyNoKill(t *testing.T) {
	eng, _ := testRiskEngine(t)
	if divs := eng.reconcilePositions(); len(divs) != 0 {
		t.Fatalf("empty state must not diverge, got %v", divs)
	}
	if eng.killSwitch.Load() {
		t.Fatal("empty reconcile must not trip the kill switch")
	}
}

// Injected divergence: the ledger says 1.0, the position key says 2.0.
// The system must halt (kill) and page (risk.alerts), not silently agree.
func TestReconcileDivergenceHaltsAndPages(t *testing.T) {
	eng, fr := testRiskEngine(t)
	seedFill(t, fr, "BTCUSDT", "BUY", 1.0)
	seedPosition(t, fr, "BTCUSDT", 2.0, true) // tampered / double-counted

	divs := eng.reconcilePositions()
	if len(divs) == 0 {
		t.Fatal("injected divergence must be detected")
	}
	if !eng.killSwitch.Load() {
		t.Fatal("divergence must fail closed: kill switch not active")
	}
	if got := fr.Get(context.Background(), "kill_switch").Val(); got != "1" {
		t.Fatalf("durable kill flag = %q, want 1", got)
	}
	// The divergence is audited on the three-layer path.
	found := false
	for _, e := range fr.streamEntries(controlAuditStream) {
		if e["action"] == "risk.gate8" && e["outcome"] == "divergence" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("divergence must be audited (risk.gate8/divergence)")
	}
}

// The count/set must agree with the quantities: a symbol sitting in
// open_symbols with a zero position is the old never-decrement bug in a new
// costume, and must also fail closed.
func TestReconcileSetInconsistencyHalts(t *testing.T) {
	eng, fr := testRiskEngine(t)
	seedPosition(t, fr, "BTCUSDT", 0, true) // zero qty but counted as open

	if divs := eng.reconcilePositions(); len(divs) == 0 {
		t.Fatal("set/quantity inconsistency must be detected")
	}
	if !eng.killSwitch.Load() {
		t.Fatal("set inconsistency must fail closed: kill switch not active")
	}
}

// Gate 8 now reads the idempotent open_symbols set, not the old drift-only
// counter.
func TestGate8ReadsOpenSymbolsSet(t *testing.T) {
	eng, fr := testRiskEngine(t)
	ctx := context.Background()
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().UnixMilli()))
	for i := 0; i < 8; i++ {
		if err := fr.SAdd(ctx, "portfolio:open_symbols", fmt.Sprintf("SYM%d", i)).Err(); err != nil {
			t.Fatal(err)
		}
	}
	ok, reason, _ := eng.validate(gate5Signal("sig-gate8-1"))
	if ok || reason != "GATE8_MAX_POSITIONS_REACHED" {
		t.Fatalf("got ok=%t reason=%q, want GATE8_MAX_POSITIONS_REACHED", ok, reason)
	}
}

// The stale legacy counter key must NOT drive Gate 8 anymore — even if some
// old writer left a value there, the set is authoritative.
func TestGate8IgnoresLegacyCounter(t *testing.T) {
	eng, fr := testRiskEngine(t)
	ctx := context.Background()
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().UnixMilli()))
	if err := fr.Set(ctx, "portfolio:open_positions", "8", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := fr.SAdd(ctx, "portfolio:open_symbols", "BTCUSDT").Err(); err != nil {
		t.Fatal(err)
	}
	ok, reason, _ := eng.validate(gate5Signal("sig-gate8-2"))
	if !ok {
		t.Fatalf("legacy counter must not gate: reason=%q", reason)
	}
}
