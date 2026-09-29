package main

// Phase 1B.1 — fail-closed freshness (Gate 5).
//
// Failure mode under test: the market feed goes silent (or publishes
// synthetic timestamp:0 data) and the risk engine must refuse new intents
// within one tick instead of approving against a stale/missing price.
//
// Run: go test -run 'TestGate5' -v

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func testRiskEngine(t *testing.T) (*RiskEngine, *fakeRedis) {
	t.Helper()
	t.Setenv("PAPER_MODE", "true")
	t.Setenv("TRUTHCORE_URL", "http://127.0.0.1:1") // fail fast, no network
	fr := newFakeRedis()
	// Mirror NewRiskEngine defaults: a zero-value RiskEngine has 0
	// thresholds, which would trip gates unrelated to freshness.
	return &RiskEngine{
		redis:            fr,
		seenSignalIDs:    make(map[string]struct{}),
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
	}, fr
}

func gate5Signal(id string) map[string]interface{} {
	return map[string]interface{}{
		"signal_id":   id,
		"strategy_id": "test-strategy",
		"symbol":      "BTCUSDT",
		"mode":        "paper",
		"limit_price": 63000.0,
		"quantity":    0.01,
		"stop":        62000.0,
		"confidence":  0.90,
	}
}

func setBookTS(t *testing.T, fr *fakeRedis, symbol, value string) {
	t.Helper()
	if err := fr.Set(context.Background(), "book_ts:"+symbol, value, 0).Err(); err != nil {
		t.Fatal(err)
	}
}

// The exact failure mode from the Phase 0 audit: book_ts:<symbol> is never
// written, so the old code skipped the staleness check entirely and approved.
func TestGate5MissingKeyFailsClosed(t *testing.T) {
	eng, fr := testRiskEngine(t)
	ok, reason, _ := eng.validate(gate5Signal("sig-missing-1"))
	if ok {
		t.Fatal("validate must not approve with no freshness key")
	}
	if reason != "GATE5_NO_BOOK_TS" {
		t.Fatalf("reason = %q, want GATE5_NO_BOOK_TS", reason)
	}
	assertGate5Audited(t, fr, "GATE5_NO_BOOK_TS")
}

func TestGate5ZeroTimestampFailsClosed(t *testing.T) {
	eng, fr := testRiskEngine(t)
	setBookTS(t, fr, "BTCUSDT", "0") // synthetic feed: timestamp: 0
	ok, reason, _ := eng.validate(gate5Signal("sig-zero-1"))
	if ok {
		t.Fatal("validate must not approve a zero timestamp")
	}
	if reason != "GATE5_ZERO_BOOK_TS" {
		t.Fatalf("reason = %q, want GATE5_ZERO_BOOK_TS", reason)
	}
	assertGate5Audited(t, fr, "GATE5_ZERO_BOOK_TS")
}

func TestGate5BadTimestampFailsClosed(t *testing.T) {
	eng, _ := testRiskEngine(t)
	fr := eng.redis.(*fakeRedis)
	setBookTS(t, fr, "BTCUSDT", "not-a-number")
	ok, reason, _ := eng.validate(gate5Signal("sig-bad-1"))
	if ok || reason != "GATE5_BAD_BOOK_TS" {
		t.Fatalf("got ok=%t reason=%q, want GATE5_BAD_BOOK_TS", ok, reason)
	}
}

func TestGate5FutureTimestampFailsClosed(t *testing.T) {
	eng, _ := testRiskEngine(t)
	fr := eng.redis.(*fakeRedis)
	future := fmt.Sprintf("%d", time.Now().Add(2*time.Minute).UnixMilli())
	setBookTS(t, fr, "BTCUSDT", future)
	ok, reason, _ := eng.validate(gate5Signal("sig-future-1"))
	if ok || !strings.HasPrefix(reason, "GATE5_FUTURE_BOOK_TS_") {
		t.Fatalf("got ok=%t reason=%q, want GATE5_FUTURE_BOOK_TS_*", ok, reason)
	}
}

// Injected-stale-feed test: the feed goes silent and the very next validate
// — one tick — refuses the intent.
func TestGate5StaleFeedHaltsWithinOneTick(t *testing.T) {
	eng, fr := testRiskEngine(t)
	// Feed was live 60s ago and then went silent.
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().Add(-60*time.Second).UnixMilli()))
	ok, reason, _ := eng.validate(gate5Signal("sig-stale-1"))
	if ok {
		t.Fatal("stale feed must halt new intents within one tick")
	}
	if !strings.HasPrefix(reason, "GATE5_STALE_BOOK_") {
		t.Fatalf("reason = %q, want GATE5_STALE_BOOK_*", reason)
	}
	assertGate5Audited(t, fr, "GATE5_STALE_BOOK_")
}

func TestGate5FreshBookPasses(t *testing.T) {
	eng, _ := testRiskEngine(t)
	fr := eng.redis.(*fakeRedis)
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().UnixMilli()))
	ok, reason, _ := eng.validate(gate5Signal("sig-fresh-1"))
	if !ok {
		t.Fatalf("fresh book must pass gate 5, got reason=%q", reason)
	}
	if reason != "APPROVED" {
		t.Fatalf("reason = %q, want APPROVED", reason)
	}
}

func TestGate5PerSymbolOverride(t *testing.T) {
	eng, _ := testRiskEngine(t)
	fr := eng.redis.(*fakeRedis)
	t.Setenv("STALE_BOOK_SECONDS_BTCUSDT", "60")
	// 30s old: stale under the 5s default, fresh under the 60s override.
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().Add(-30*time.Second).UnixMilli()))
	ok, reason, _ := eng.validate(gate5Signal("sig-override-1"))
	if !ok {
		t.Fatalf("per-symbol override must apply, got reason=%q", reason)
	}
}

// Standing rule: EVERY fail-closed event is audited, no exceptions, no
// aggregation. Three rejections → three audit events.
func TestGate5EveryRejectionAudited(t *testing.T) {
	eng, fr := testRiskEngine(t)
	for i := 0; i < 3; i++ {
		ok, _, _ := eng.validate(gate5Signal(fmt.Sprintf("sig-noagg-%d", i)))
		if ok {
			t.Fatal("must stay rejected during outage")
		}
	}
	var n int
	for _, e := range fr.streamEntries(controlAuditStream) {
		if e["action"] == "risk.gate5" && e["outcome"] == "fail_closed" {
			n++
			if e["audit_id"] == nil || e["audit_id"] == "" {
				t.Fatal("every fail-closed audit event must carry an audit_id")
			}
		}
	}
	if n != 3 {
		t.Fatalf("expected 3 fail-closed audit events (one per rejection), got %d", n)
	}
}

func assertGate5Audited(t *testing.T, fr *fakeRedis, reasonSubstr string) {
	t.Helper()
	for _, e := range fr.streamEntries(controlAuditStream) {
		if e["action"] == "risk.gate5" && e["outcome"] == "fail_closed" &&
			strings.Contains(e["reason"].(string), reasonSubstr) {
			if e["audit_id"] == nil || e["audit_id"] == "" {
				t.Fatal("fail-closed audit event must carry an audit_id")
			}
			return
		}
	}
	t.Fatalf("no fail-closed audit event for %q", reasonSubstr)
}
