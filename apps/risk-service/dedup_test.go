package main

// Phase 1B.3 — durable dedup (Gate 11).
//
// Failure mode under test: risk-service approves a signal, then crashes
// before the approved signal is published (crash between submit and
// record). On recovery the same signal is redelivered — it must be rejected
// as a duplicate, never approved twice. The old in-memory seen-set lost all
// state on restart (and wiped itself every 50k entries); the new record is
// an atomic durable SET NX written before publish.
//
// Run: go test -run 'TestDedup' -v

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func dedupSignal(id string) map[string]interface{} {
	s := gate5Signal(id)
	s["signal_id"] = id
	return s
}

func seedFreshBook(t *testing.T, fr *fakeRedis) {
	t.Helper()
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().UnixMilli()))
}

func TestDedupFirstSeenPassesDuplicateRejected(t *testing.T) {
	eng, fr := testRiskEngine(t)
	seedFreshBook(t, fr)

	ok, reason, _ := eng.validate(dedupSignal("dup-1"))
	if !ok {
		t.Fatalf("first sight must pass, got %q", reason)
	}
	ok, reason, _ = eng.validate(dedupSignal("dup-1"))
	if ok || reason != "GATE11_DUPLICATE_SIGNAL_ID" {
		t.Fatalf("got ok=%t reason=%q, want GATE11_DUPLICATE_SIGNAL_ID", ok, reason)
	}
}

func TestDedupSurvivesRestart(t *testing.T) {
	fr := newFakeRedis()
	newEngine := func() *RiskEngine {
		t.Setenv("PAPER_MODE", "true")
		t.Setenv("TRUTHCORE_URL", "http://127.0.0.1:1")
		return &RiskEngine{
			redis:            fr,
			maxDailyLoss:     0.02,
			maxDrawdown:      0.10,
			maxPositions:     8,
			maxNotional:      50_000,
			maxLeverage:      2.0,
			maxSpreadBps:     20,
			riskPerTrade:     0.005,
			minConfidence:    0.60,
			staleBookSecs:    5,
			maxAssetExposure: 0.25,
			maxCorrelation:   0.70,
			dedupTTL:         time.Hour,
		}
	}

	eng1 := newEngine()
	seedFreshBook(t, fr)
	ok, _, _ := eng1.validate(dedupSignal("restart-1"))
	if !ok {
		t.Fatal("first engine must approve")
	}

	// Simulate a process restart: brand-new engine, same durable Redis.
	eng2 := newEngine()
	ok, reason, _ := eng2.validate(dedupSignal("restart-1"))
	if ok || reason != "GATE11_DUPLICATE_SIGNAL_ID" {
		t.Fatalf("post-restart redelivery: got ok=%t reason=%q, want duplicate rejection", ok, reason)
	}
}

// The exact 1B.3 scenario: validate() approves (durably recording the ID),
// the process crashes BEFORE forward("signals.approved") ever runs, and the
// redelivered signal must still be rejected — no double order.
func TestDedupCrashBetweenSubmitAndRecord(t *testing.T) {
	fr := newFakeRedis()
	t.Setenv("PAPER_MODE", "true")
	t.Setenv("TRUTHCORE_URL", "http://127.0.0.1:1")
	eng := &RiskEngine{redis: fr, maxDailyLoss: 0.02, maxDrawdown: 0.10,
		maxPositions: 8, maxNotional: 50_000, maxLeverage: 2.0, maxSpreadBps: 20,
		riskPerTrade: 0.005, minConfidence: 0.60, staleBookSecs: 5,
		maxAssetExposure: 0.25, maxCorrelation: 0.70, dedupTTL: time.Hour}
	seedFreshBook(t, fr)

	ok, _, _ := eng.validate(dedupSignal("crash-1"))
	if !ok {
		t.Fatal("must approve before the simulated crash")
	}
	// The dedup record is durable BEFORE any publish: the crash window
	// between "submit to Kafka" and "record the ID" no longer exists.
	if got := fr.Get(context.Background(), "risk:seen_signal:crash-1").Val(); got != "1" {
		t.Fatal("dedup record must exist in Redis immediately after validate, before publish")
	}

	// Crash: drop the engine without publishing. Restart on the same Redis.
	eng2 := &RiskEngine{redis: fr, maxDailyLoss: 0.02, maxDrawdown: 0.10,
		maxPositions: 8, maxNotional: 50_000, maxLeverage: 2.0, maxSpreadBps: 20,
		riskPerTrade: 0.005, minConfidence: 0.60, staleBookSecs: 5,
		maxAssetExposure: 0.25, maxCorrelation: 0.70, dedupTTL: time.Hour}
	ok, reason, _ := eng2.validate(dedupSignal("crash-1"))
	if ok || reason != "GATE11_DUPLICATE_SIGNAL_ID" {
		t.Fatalf("redelivery after crash: got ok=%t reason=%q, want duplicate rejection", ok, reason)
	}
}

// Fail-closed: if the dedup store is unavailable we cannot prove uniqueness,
// so the signal is rejected rather than risked as a duplicate.
func TestDedupStoreDownFailsClosed(t *testing.T) {
	eng, fr := testRiskEngine(t)
	seedFreshBook(t, fr)
	fr.setFailSetNX(errors.New("redis down"))

	ok, reason, _ := eng.validate(dedupSignal("store-down-1"))
	if ok || reason != "GATE11_DEDUP_STORE_UNAVAILABLE" {
		t.Fatalf("got ok=%t reason=%q, want GATE11_DEDUP_STORE_UNAVAILABLE", ok, reason)
	}
}

// Atomicity: N concurrent validations of the same signal ID must yield
// exactly one approval — no check-then-set race.
func TestDedupConcurrentDuplicate(t *testing.T) {
	eng, fr := testRiskEngine(t)
	seedFreshBook(t, fr)

	const n = 32
	var approvals atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, _, _ := eng.validate(dedupSignal(fmt.Sprintf("conc-%d", 0)))
			if ok {
				approvals.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := approvals.Load(); got != 1 {
		t.Fatalf("concurrent duplicate: %d approvals, want exactly 1", got)
	}
}
