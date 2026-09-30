package main

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/omega-prime-delta/approval"
)

// Adversarial tests for the AEGIS→VULTURE authority boundary: every order
// must carry a signed, unexpired, single-use approval whose bound fields
// exactly match the signal. Anything else → refuse, never submit.

// testRig builds an ExecutionEngine backed by in-memory Redis, holding the
// AEGIS public key. It returns the matching private key for signing.
func testRig(t *testing.T) (*ExecutionEngine, ed25519.PrivateKey, *miniredis.Miniredis) {
	t.Helper()
	t.Setenv("PAPER_MODE", "true")
	pub, priv, err := approval.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	e := &ExecutionEngine{redis: rdb, approvalPub: pub, orders: make(map[string]*Order)}
	return e, priv, s
}

// deadRig builds an engine whose Redis is unreachable: every Redis error
// must fail closed.
func deadRig(t *testing.T) (*ExecutionEngine, ed25519.PrivateKey) {
	t.Helper()
	t.Setenv("PAPER_MODE", "true")
	pub, priv, err := approval.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}) // nothing listens here
	return &ExecutionEngine{redis: rdb, approvalPub: pub}, priv
}

// signedSignal builds a signal carrying an AEGIS approval, exactly as
// risk-service would emit it (approval signed over the final fields).
func signedSignal(priv ed25519.PrivateKey, signTime time.Time) map[string]interface{} {
	a := &approval.Approval{
		ApprovalID: "appr-test-1",
		SignalID:   "sig-test-1",
		StrategyID: "strat-test",
		Symbol:     "BTC/USD",
		Side:       "BUY",
		Quantity:   0.01,
		LimitPrice: 63000,
		StopPrice:  62000,
		Mode:       "paper",
	}
	a.Sign(priv, signTime)
	return map[string]interface{}{
		"signal_id":      a.SignalID,
		"strategy_id":    a.StrategyID,
		"symbol":         a.Symbol,
		"side":           a.Side,
		"quantity":       a.Quantity,
		"limit_price":    a.LimitPrice,
		"stop":           a.StopPrice,
		"mode":           a.Mode,
		"aegis_approval": a.ToMap(),
	}
}

// A valid approval verifies and is claimed exactly once.
func TestVerifyApproval_AcceptsValid(t *testing.T) {
	e, priv, _ := testRig(t)
	signal := signedSignal(priv, time.Now())
	appr, reason := e.verifyApproval(signal)
	if reason != "" {
		t.Fatalf("valid approval refused: %s", reason)
	}
	if appr.ApprovalID != "appr-test-1" {
		t.Fatalf("wrong approval returned: %s", appr.ApprovalID)
	}
	// The claim record must exist in Redis (the database boundary).
	if got := e.redis.Get(context.Background(), appr.ClaimKey()).Val(); got != "claimed" {
		t.Fatalf("claim record missing in redis: %q", got)
	}
}

// The same approval presented twice is a replay → refuse.
func TestVerifyApproval_RejectsReplay(t *testing.T) {
	e, priv, _ := testRig(t)
	signal := signedSignal(priv, time.Now())
	if _, reason := e.verifyApproval(signal); reason != "" {
		t.Fatalf("first presentation refused: %s", reason)
	}
	// A fresh signal map with the same approval (as a re-delivered Kafka
	// message would look) must still be caught.
	signal2 := signedSignal(priv, time.Now())
	if _, reason := e.verifyApproval(signal2); reason != "APPROVAL_REPLAY" {
		t.Fatalf("replay not rejected: got %q", reason)
	}
}

// Mutating ANY execution-critical field after signing breaks the binding.
func TestVerifyApproval_RejectsTamperedFields(t *testing.T) {
	tampers := map[string]func(m map[string]interface{}){
		"quantity":    func(m map[string]interface{}) { m["quantity"] = 999.0 },
		"limit_price": func(m map[string]interface{}) { m["limit_price"] = 1.0 },
		"stop":        func(m map[string]interface{}) { m["stop"] = 1.0 },
		"side":        func(m map[string]interface{}) { m["side"] = "SELL" },
		"symbol":      func(m map[string]interface{}) { m["symbol"] = "ETH/USD" },
		"signal_id":   func(m map[string]interface{}) { m["signal_id"] = "sig-evil" },
		"mode":        func(m map[string]interface{}) { m["mode"] = "live" },
	}
	for name, tamper := range tampers {
		e, priv, _ := testRig(t)
		signal := signedSignal(priv, time.Now())
		tamper(signal)
		if _, reason := e.verifyApproval(signal); reason != "APPROVAL_FIELD_MISMATCH" {
			t.Fatalf("tamper %s: got %q, want APPROVAL_FIELD_MISMATCH", name, reason)
		}
	}
}

// A corrupted signature fails verification.
func TestVerifyApproval_RejectsBadSignature(t *testing.T) {
	e, priv, _ := testRig(t)
	signal := signedSignal(priv, time.Now())
	apprMap := signal["aegis_approval"].(map[string]interface{})
	apprMap["signature"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
	if _, reason := e.verifyApproval(signal); reason != "APPROVAL_BAD_SIGNATURE" {
		t.Fatalf("corrupted signature: got %q", reason)
	}
}

// An approval signed by an unknown key fails verification.
func TestVerifyApproval_RejectsUnknownKey(t *testing.T) {
	e, _, _ := testRig(t)
	_, evilPriv, _ := approval.GenerateKeypair()
	signal := signedSignal(evilPriv, time.Now())
	if _, reason := e.verifyApproval(signal); reason != "APPROVAL_BAD_SIGNATURE" {
		t.Fatalf("unknown key: got %q", reason)
	}
}

// An expired approval fails, even though its signature is valid.
func TestVerifyApproval_RejectsExpired(t *testing.T) {
	e, priv, _ := testRig(t)
	signal := signedSignal(priv, time.Now().Add(-2*time.Minute))
	if _, reason := e.verifyApproval(signal); reason != "APPROVAL_EXPIRED" {
		t.Fatalf("expired approval: got %q", reason)
	}
}

// No approval attached → refuse. (This is what a raw injection into the
// signals.approved topic looks like.)
func TestVerifyApproval_RejectsMissing(t *testing.T) {
	e, _, _ := testRig(t)
	signal := map[string]interface{}{"signal_id": "sig-naked", "symbol": "BTC/USD"}
	if _, reason := e.verifyApproval(signal); reason != "APPROVAL_MISSING" {
		t.Fatalf("missing approval: got %q", reason)
	}
}

// Redis down at claim time → fail closed, never submit.
func TestVerifyApproval_ClaimStoreDownFailsClosed(t *testing.T) {
	e, priv := deadRig(t)
	signal := signedSignal(priv, time.Now())
	if _, reason := e.verifyApproval(signal); reason != "APPROVAL_CLAIM_STORE_UNAVAILABLE" {
		t.Fatalf("claim store down: got %q", reason)
	}
}

// Kill-switch state unreadable → UNKNOWN → refuse (the old .Val() failed open).
func TestKillActiveAtSubmit_RedisDownIsUnknown(t *testing.T) {
	e, _ := deadRig(t)
	active, unknown := e.killActiveAtSubmit()
	if !unknown || active {
		t.Fatalf("redis down: got active=%v unknown=%v, want active=false unknown=true", active, unknown)
	}
}

// Kill switch set → active.
func TestKillActiveAtSubmit_RespectsKillFlag(t *testing.T) {
	e, _, s := testRig(t)
	active, unknown := e.killActiveAtSubmit()
	if active || unknown {
		t.Fatalf("no kill flag: got active=%v unknown=%v", active, unknown)
	}
	s.Set("kill_switch", "1")
	active, unknown = e.killActiveAtSubmit()
	if !active || unknown {
		t.Fatalf("kill flag set: got active=%v unknown=%v", active, unknown)
	}
}

// End-to-end through processSignal: kill active at submit → cancelled with
// the kill reason, never routed.
func TestProcessSignal_RefusesWhenKilled(t *testing.T) {
	e, priv, s := testRig(t)
	s.Set("kill_switch", "1")
	signal := signedSignal(priv, time.Now())
	e.processSignal(signal)
	if len(e.orders) != 1 {
		t.Fatalf("expected 1 refused order, got %d", len(e.orders))
	}
	for _, o := range e.orders {
		if o.State != StateCancelled {
			t.Fatalf("order state = %s, want cancelled", o.State)
		}
		if o.Meta["cancel_reason"] != "KILL_SWITCH_ACTIVE_AT_SUBMIT" {
			t.Fatalf("cancel_reason = %v", o.Meta["cancel_reason"])
		}
	}
}
