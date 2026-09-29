package main

// Phase 1B.2 — the open-position predicate used by updatePortfolio's
// idempotent set maintenance. A close (net qty back to ~0) must remove the
// symbol from portfolio:open_symbols on the same fill event.

import "testing"

func TestIsOpenPosition(t *testing.T) {
	cases := []struct {
		qty  float64
		want bool
	}{
		{0, false},
		{1e-10, false}, // dust → closed
		{-1e-10, false},
		{1e-8, true},
		{0.01, true},
		{-2.5, true}, // short is still an open position
	}
	for _, c := range cases {
		if got := isOpenPosition(c.qty); got != c.want {
			t.Errorf("isOpenPosition(%v) = %t, want %t", c.qty, got, c.want)
		}
	}
}
