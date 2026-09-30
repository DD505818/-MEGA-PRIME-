package main

// PAPER/LIVE lock — Gate 3 canonical mode tests.
//
// Failure mode under test: a signal declaring a non-paper mode (or the
// process itself running outside paper mode) must never validate. Mode
// interpretation is canonical via modelock; Gate 3 enforces it per-signal.

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A live-mode signal must be rejected at Gate 3, before any other gate.
func TestGate3LiveSignalRejected(t *testing.T) {
	eng, _ := testRiskEngine(t)
	sig := gate5Signal("sig-g3-live")
	sig["mode"] = "live"
	ok, reason, _ := eng.validate(sig)
	if ok {
		t.Fatal("validate must not approve a live-mode signal")
	}
	if reason != "GATE3_SIGNAL_MODE_MISMATCH" {
		t.Fatalf("reason = %q, want GATE3_SIGNAL_MODE_MISMATCH", reason)
	}
}

// Case-insensitive variants must also be rejected.
func TestGate3LiveSignalCaseVariantsRejected(t *testing.T) {
	for _, mode := range []string{"LIVE", "Live", " live ", "REAL"} {
		eng, _ := testRiskEngine(t)
		sig := gate5Signal("sig-g3-live-" + strings.TrimSpace(mode))
		sig["mode"] = mode
		ok, reason, _ := eng.validate(sig)
		if ok || reason != "GATE3_SIGNAL_MODE_MISMATCH" {
			t.Fatalf("mode %q: ok=%v reason=%q, want rejection with GATE3_SIGNAL_MODE_MISMATCH", mode, ok, reason)
		}
	}
}

// A paper-mode signal must pass Gate 3 (later gates may still reject for
// their own reasons — here we give it a fresh book so Gate 5 passes too,
// and assert Gate 3 is never the cause of rejection).
func TestGate3PaperSignalNotRejectedByGate3(t *testing.T) {
	eng, fr := testRiskEngine(t)
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().UnixMilli()))
	sig := gate5Signal("sig-g3-paper")
	sig["mode"] = "paper"
	ok, reason, _ := eng.validate(sig)
	if !ok && strings.HasPrefix(reason, "GATE3") {
		t.Fatalf("paper-mode signal rejected by Gate 3: %q", reason)
	}
}

// An empty mode (legacy signals) must not trip Gate 3.
func TestGate3EmptyModeNotRejectedByGate3(t *testing.T) {
	eng, fr := testRiskEngine(t)
	setBookTS(t, fr, "BTCUSDT", fmt.Sprintf("%d", time.Now().UnixMilli()))
	sig := gate5Signal("sig-g3-empty")
	delete(sig, "mode")
	ok, reason, _ := eng.validate(sig)
	if !ok && strings.HasPrefix(reason, "GATE3") {
		t.Fatalf("empty-mode signal rejected by Gate 3: %q", reason)
	}
}
