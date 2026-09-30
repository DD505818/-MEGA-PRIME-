package main

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/omega-prime-delta/approval"
	"github.com/omega-prime-delta/truthclient"
)

// Adversarial tests for the Phase 4 audit-or-no-trade gates in
// execution-service:
//  1. No order is submitted unless its vulture.order_submitted record is
//     durably appended to TruthCore (AUDIT_UNAVAILABLE refusal).
//  2. Every fill is recorded (vulture.fill); a failed fill audit marks the
//     order audit_gap and halts all new submissions (AUDIT_HALTED).
//  3. Refusals are best-effort audited and never blocked by TruthCore.

// fakeTruthCore is a minimal truth-core stub: it records appended events and
// can be told to fail appends.
type fakeTruthCore struct {
	mu       sync.Mutex
	events   []appendedEvent
	failNext bool // fail the next /append with 500
	failAll  bool // fail every /append with 500
}

type appendedEvent struct {
	EventType string
	Payload   map[string]interface{}
}

func (f *fakeTruthCore) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failAll || f.failNext {
			f.failNext = false
			http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
			return
		}
		var p map[string]interface{}
		_ = json.Unmarshal(body.Payload, &p)
		f.events = append(f.events, appendedEvent{EventType: body.EventType, Payload: p})
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "entry_id": "e1", "event_type": body.EventType,
			"payload": body.Payload, "prev_hash": "genesis", "hash": "abc",
		})
	})
}

func runFakeTruthCore(t *testing.T, f *fakeTruthCore) string {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return srv.URL
}

func auditedRig(t *testing.T, f *fakeTruthCore) (*ExecutionEngine, ed25519.PrivateKey) {
	t.Helper()
	e, priv, _ := testRig(t)
	e.truthClient = truthclient.New(runFakeTruthCore(t, f), "")
	return e, priv
}

func eventTypes(f *fakeTruthCore) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, e := range f.events {
		out = append(out, e.EventType)
	}
	return out
}

func lastEvent(f *fakeTruthCore) appendedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[len(f.events)-1]
}

// signedSignalWithID builds a properly signed signal with the given IDs,
// for tests that need more than one distinct single-use approval.
func signedSignalWithID(priv ed25519.PrivateKey, signTime time.Time, approvalID, signalID string) map[string]interface{} {
	a := &approval.Approval{
		ApprovalID: approvalID,
		SignalID:   signalID,
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

// A nil truth client refuses the order: audit-or-no-trade, fail closed.
func TestProcessSignal_RefusesWhenAuditUnavailable(t *testing.T) {
	e, priv, _ := testRig(t) // truthClient nil
	signal := signedSignal(priv, time.Now())
	e.processSignal(signal)
	if len(e.orders) != 1 {
		t.Fatalf("expected 1 refused order, got %d", len(e.orders))
	}
	for _, o := range e.orders {
		if o.State != StateCancelled {
			t.Fatalf("order state = %s, want cancelled", o.State)
		}
		if o.Meta["cancel_reason"] != "AUDIT_UNAVAILABLE" {
			t.Fatalf("cancel_reason = %v, want AUDIT_UNAVAILABLE", o.Meta["cancel_reason"])
		}
	}
}

// An unreachable TruthCore refuses the order.
func TestProcessSignal_RefusesWhenTruthCoreDown(t *testing.T) {
	e, priv, _ := testRig(t)
	e.truthClient = truthclient.New("http://127.0.0.1:1", "")
	signal := signedSignal(priv, time.Now())
	e.processSignal(signal)
	for _, o := range e.orders {
		if o.Meta["cancel_reason"] != "AUDIT_UNAVAILABLE" {
			t.Fatalf("cancel_reason = %v, want AUDIT_UNAVAILABLE", o.Meta["cancel_reason"])
		}
	}
}

// The happy path records order_submitted before routing and the fill after,
// with matching order_id and approval_id lineage.
func TestProcessSignal_AuditsOrderAndFill(t *testing.T) {
	f := &fakeTruthCore{}
	e, priv, _ := testRig(t)
	e.truthClient = truthclient.New(runFakeTruthCore(t, f), "")
	signal := signedSignal(priv, time.Now())
	e.processSignal(signal)

	if got := eventTypes(f); len(got) != 2 ||
		got[0] != truthclient.EventOrderSubmitted ||
		got[1] != truthclient.EventFill {
		t.Fatalf("audited events = %v, want [order_submitted fill]", got)
	}

	var orderID string
	for id, o := range e.orders {
		orderID = id
		if o.State != StateFilled {
			t.Fatalf("order state = %s, want filled", o.State)
		}
		if o.Meta["audit_gap"] == true {
			t.Fatal("order incorrectly marked audit_gap")
		}
	}
	sub := f.events[0].Payload
	fill := f.events[1].Payload
	if sub["order_id"] != orderID || fill["order_id"] != orderID {
		t.Fatalf("order_id not bound: sub=%v fill=%v want %s", sub["order_id"], fill["order_id"], orderID)
	}
	if sub["approval_id"] != "appr-test-1" || fill["approval_id"] != "appr-test-1" {
		t.Fatalf("approval_id not bound: sub=%v fill=%v", sub["approval_id"], fill["approval_id"])
	}
	if sub["quantity"] != 0.01 || fill["quantity"] != 0.01 {
		t.Fatalf("quantity not bound: sub=%v fill=%v", sub["quantity"], fill["quantity"])
	}
}

// A failed fill audit marks the gap and halts all new submissions.
func TestProcessSignal_FillAuditGapHalts(t *testing.T) {
	// Fake truth-core that accepts everything except vulture.fill.
	failed := &fakeTruthCore{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.EventType == truthclient.EventFill {
			http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
			return
		}
		var p map[string]interface{}
		_ = json.Unmarshal(body.Payload, &p)
		failed.mu.Lock()
		failed.events = append(failed.events, appendedEvent{EventType: body.EventType, Payload: p})
		failed.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	}))
	t.Cleanup(srv.Close)

	e, priv, _ := testRig(t)
	e.truthClient = truthclient.New(srv.URL, "")

	e.processSignal(signedSignal(priv, time.Now()))
	var filled *Order
	for _, o := range e.orders {
		filled = o
	}
	if filled.State != StateFilled {
		t.Fatalf("order state = %s, want filled (fill happened, audit failed)", filled.State)
	}
	if filled.Meta["audit_gap"] != true {
		t.Fatal("order not marked audit_gap after failed fill audit")
	}
	if !e.auditHalted.Load() {
		t.Fatal("auditHalted not set after failed fill audit")
	}

	// The next signal (with a FRESH, properly signed approval) must be
	// refused — no new trade without audit.
	e.processSignal(signedSignalWithID(priv, time.Now(), "appr-test-2", "sig-test-2"))
	for _, o := range e.orders {
		if o == filled {
			continue
		}
		if o.Meta["cancel_reason"] != "AUDIT_HALTED" {
			t.Fatalf("second order cancel_reason = %v, want AUDIT_HALTED", o.Meta["cancel_reason"])
		}
		return
	}
	t.Fatal("second order was not refused")
}
