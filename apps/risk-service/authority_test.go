package main

import (
	"testing"
	"time"

	"github.com/omega-prime-delta/approval"
)

// The issuer side of the authority boundary: an approved signal gets a
// signed approval binding the risk-ADJUSTED quantity, verifiable by VULTURE.

// issueApproval signs the adjusted quantity and binds every execution-critical field.
func TestIssueApproval_BindsAdjustedQuantity(t *testing.T) {
	engine, _ := testRiskEngine(t)
	pub, priv, err := approval.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	_ = pub
	engine.approvalPriv = priv

	signal := map[string]interface{}{
		"signal_id":   "sig-1",
		"strategy_id": "strat-1",
		"symbol":      "BTC/USD",
		"side":        "BUY",
		"quantity":    0.037, // risk-adjusted value, as run() sets before signing
		"limit_price": 63000.5,
		"stop":        62000.0,
		"mode":        "paper",
	}
	appr := engine.issueApproval(signal)
	if err := appr.Verify(pub); err != nil {
		t.Fatalf("issued approval does not verify: %v", err)
	}
	if appr.Quantity != 0.037 {
		t.Fatalf("approval quantity = %v, want the adjusted 0.037", appr.Quantity)
	}
	for field, want := range map[string]string{
		"signal_id": "sig-1", "strategy_id": "strat-1", "symbol": "BTC/USD",
		"side": "BUY", "mode": "paper",
	} {
		var got string
		switch field {
		case "signal_id":
			got = appr.SignalID
		case "strategy_id":
			got = appr.StrategyID
		case "symbol":
			got = appr.Symbol
		case "side":
			got = appr.Side
		case "mode":
			got = appr.Mode
		}
		if got != want {
			t.Fatalf("approval.%s = %q, want %q", field, got, want)
		}
	}
	if appr.LimitPrice != 63000.5 || appr.StopPrice != 62000.0 {
		t.Fatalf("prices not bound: %+v", appr)
	}
	if appr.Expired(time.Now()) {
		t.Fatal("fresh approval reports expired")
	}
	if appr.ApprovalID == "" {
		t.Fatal("approval has no ID")
	}
}

// The approval survives the Kafka round-trip (map → JSON → map) that run()
// performs via forward().
func TestIssueApproval_SurvivesForwardRoundTrip(t *testing.T) {
	engine, _ := testRiskEngine(t)
	pub, priv, err := approval.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	engine.approvalPriv = priv
	signal := map[string]interface{}{
		"signal_id": "sig-2", "strategy_id": "strat-1", "symbol": "BTC/USD",
		"side": "BUY", "quantity": 0.01, "limit_price": 63000.0,
		"stop": 62000.0, "mode": "paper",
	}
	appr := engine.issueApproval(signal)
	// Simulate forward(): marshal the signal, unmarshal, extract approval.
	roundTripped, err := approval.FromMap(appr.ToMap())
	if err != nil {
		t.Fatalf("round-trip decode failed: %v", err)
	}
	if err := roundTripped.Verify(pub); err != nil {
		t.Fatalf("round-tripped approval does not verify: %v", err)
	}
}
