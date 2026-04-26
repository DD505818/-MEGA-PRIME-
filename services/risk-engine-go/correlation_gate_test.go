package main

import "testing"

func TestCorrelationGateAllowsNoExistingPositions(t *testing.T) {
	gate := NewStreamingCorrelationMatrix(8, 0.70)
	decision := gate.EvaluateAdd("BTCUSD", nil)

	if !decision.Allowed {
		t.Fatalf("expected candidate allowed with no existing positions, got %s", decision.Reason)
	}
	if decision.Reason != "NO_EXISTING_POSITIONS" {
		t.Fatalf("expected NO_EXISTING_POSITIONS, got %s", decision.Reason)
	}
}

func TestCorrelationGateRejectsInvalidSymbol(t *testing.T) {
	gate := NewStreamingCorrelationMatrix(8, 0.70)
	decision := gate.EvaluateAdd("", nil)

	if decision.Allowed {
		t.Fatal("expected invalid symbol to be rejected")
	}
	if decision.Reason != "INVALID_SYMBOL" {
		t.Fatalf("expected INVALID_SYMBOL, got %s", decision.Reason)
	}
}

func TestCorrelationGateAllowsSingleLowCorrelationPosition(t *testing.T) {
	gate := NewStreamingCorrelationMatrix(8, 0.70)
	decision := gate.EvaluateAdd("ETHUSD", []CorrelationPosition{{Symbol: "BTCUSD", Correlation: 0.40}})

	if !decision.Allowed {
		t.Fatalf("expected low-correlation candidate allowed, got %s", decision.Reason)
	}
	if decision.Reason != "CORRELATION_WITHIN_LIMIT" {
		t.Fatalf("expected CORRELATION_WITHIN_LIMIT, got %s", decision.Reason)
	}
}

func TestCorrelationGateBlocksHighAverageCorrelation(t *testing.T) {
	gate := NewStreamingCorrelationMatrix(8, 0.70)
	decision := gate.EvaluateAdd("ETHUSD", []CorrelationPosition{
		{Symbol: "BTCUSD", Correlation: 0.80},
		{Symbol: "SOLUSD", Correlation: 0.90},
	})

	if decision.Allowed {
		t.Fatal("expected high average correlation to be blocked")
	}
	if decision.Reason != "CORRELATION_LIMIT_EXCEEDED" {
		t.Fatalf("expected CORRELATION_LIMIT_EXCEEDED, got %s", decision.Reason)
	}
	if decision.AverageCorrelation <= 0.70 {
		t.Fatalf("expected avg correlation above 0.70, got %.4f", decision.AverageCorrelation)
	}
}

func TestCorrelationGateSameSymbolOnlyDoesNotFalseBlock(t *testing.T) {
	gate := NewStreamingCorrelationMatrix(8, 0.70)
	decision := gate.EvaluateAdd("BTCUSD", []CorrelationPosition{{Symbol: "BTCUSD", Correlation: 1.00}})

	if !decision.Allowed {
		t.Fatalf("expected same-symbol-only position set to avoid false block, got %s", decision.Reason)
	}
	if decision.Reason != "SAME_SYMBOL_ONLY" {
		t.Fatalf("expected SAME_SYMBOL_ONLY, got %s", decision.Reason)
	}
}
