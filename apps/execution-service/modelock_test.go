package main

// PAPER/LIVE lock tests.
//
// Failure mode under test: any code path that could reach a live venue must
// refuse loudly instead of executing or silently simulating. LIVE is locked:
// liveFill is a hard error, and the worker asserts paper mode per message.

import (
	"testing"

	"github.com/omega-prime-delta/modelock"
)

// liveFill must never fill — it is the future broker integration point and
// must stay a hard error until a certification program authorizes otherwise.
func TestLiveFillAlwaysErrors(t *testing.T) {
	e := &ExecutionEngine{paperMode: true}
	for _, typ := range []OrderType{TypeTWAP, TypeIceberg, TypeMarket, TypeLimit} {
		price, err := e.liveFill(&Order{Type: typ, LimitPrice: 63000, Qty: 0.01, Side: "BUY"}, nil)
		if err == nil {
			t.Fatalf("liveFill(%s) must error while LIVE is locked", typ)
		}
		if price != 0 {
			t.Fatalf("liveFill(%s) returned price %v; must return 0 on refusal", typ, price)
		}
	}
}

// The canonical lock must agree: paper env → paper; anything else → refusal.
func TestModelockContract(t *testing.T) {
	t.Setenv("PAPER_MODE", "true")
	t.Setenv("LIVE_TRADING_ENABLED", "false")
	if !modelock.IsPaper() {
		t.Fatal("IsPaper must be true with PAPER_MODE=true")
	}
	if err := modelock.AssertPaper(); err != nil {
		t.Fatalf("AssertPaper must pass in paper mode: %v", err)
	}

	t.Setenv("PAPER_MODE", "false")
	if modelock.IsPaper() {
		t.Fatal("IsPaper must be false with PAPER_MODE=false")
	}
	if err := modelock.AssertPaper(); err == nil {
		t.Fatal("AssertPaper must refuse outside paper mode")
	}
}
