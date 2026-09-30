package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/omega-prime-delta/approval"
	"github.com/omega-prime-delta/truthclient"
)

// Adversarial tests for the Phase 4 audit-or-no-trade gates in risk-service:
//  1. An approval is never issued without its TruthCore record: a nil or
//     unreachable truth client fails the approval (fail closed).
//  2. Independent chain verification detects tampering and engages the
//     kill switch.

// fakeTruthCore is an in-memory truth-core with the real hash algorithm.
type fakeTruthCore struct {
	mu      sync.Mutex
	entries []truthclient.Entry
	gotAuth []string // Authorization headers seen on /append
	secret  string
}

func tcHash(prev, eventType string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(prev))
	h.Write([]byte{0})
	h.Write([]byte(eventType))
	h.Write([]byte{0})
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (f *fakeTruthCore) appendLocked(eventType string, payload json.RawMessage) truthclient.Entry {
	// Canonicalize like the real server (jsonb round-trip).
	var v interface{}
	_ = json.Unmarshal(payload, &v)
	payload, _ = json.Marshal(v)
	prev := "genesis"
	var id int64 = 1
	if len(f.entries) > 0 {
		prev = f.entries[len(f.entries)-1].Hash
		id = f.entries[len(f.entries)-1].ID + 1
	}
	e := truthclient.Entry{
		ID: id, EntryID: fmt.Sprintf("e%d", id), EventType: eventType,
		Payload: payload, PrevHash: prev,
	}
	e.Hash = tcHash(e.PrevHash, e.EventType, e.Payload)
	f.entries = append(f.entries, e)
	return e
}

func (f *fakeTruthCore) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/append", func(w http.ResponseWriter, r *http.Request) {
		if f.secret != "" && r.Header.Get("Authorization") != "Bearer "+f.secret {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.gotAuth = append(f.gotAuth, r.Header.Get("Authorization"))
		var body struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		e := f.appendLocked(body.EventType, body.Payload)
		f.mu.Unlock()
		json.NewEncoder(w).Encode(e)
	})
	mux.HandleFunc("/head", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if len(f.entries) == 0 {
			http.Error(w, "empty", 404)
			return
		}
		last := f.entries[len(f.entries)-1]
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": last.ID, "entry_id": last.EntryID, "hash": last.Hash,
		})
	})
	mux.HandleFunc("/entries", func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since_id"), 10, 64)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 500
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		out := []truthclient.Entry{}
		for _, e := range f.entries {
			if e.ID > since {
				out = append(out, e)
				if len(out) >= limit {
					break
				}
			}
		}
		json.NewEncoder(w).Encode(out)
	})
	return mux
}

func runFakeTruthCore(t *testing.T, f *fakeTruthCore) string {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return srv.URL
}

func testApproval() *approval.Approval {
	return &approval.Approval{
		ApprovalID: "appr-truth-1", SignalID: "sig-1", StrategyID: "strat-1",
		Symbol: "BTC/USD", Side: "BUY", Quantity: 0.01,
		LimitPrice: 63000, StopPrice: 62000, Mode: "paper",
		IssuedAtMs: 1, GatesVersion: "v14",
	}
}

// A nil truth client fails the approval: audit-or-no-trade, fail closed.
func TestRecordApprovalIssued_NilClientFailsClosed(t *testing.T) {
	eng := &RiskEngine{redis: newFakeRedis()} // truthClient nil
	if err := eng.recordApprovalIssued(testApproval()); err == nil {
		t.Fatal("nil truth client did not fail the approval")
	}
}

// An unreachable TruthCore fails the approval.
func TestRecordApprovalIssued_UnreachableFailsClosed(t *testing.T) {
	eng := &RiskEngine{
		redis:       newFakeRedis(),
		truthClient: truthclient.New("http://127.0.0.1:1", ""), // nothing listens
	}
	if err := eng.recordApprovalIssued(testApproval()); err == nil {
		t.Fatal("unreachable truth-core did not fail the approval")
	}
}

// A healthy TruthCore records the approval with the canonical event type.
func TestRecordApprovalIssued_Success(t *testing.T) {
	f := &fakeTruthCore{}
	url := runFakeTruthCore(t, f)
	eng := &RiskEngine{redis: newFakeRedis(), truthClient: truthclient.New(url, "")}
	if err := eng.recordApprovalIssued(testApproval()); err != nil {
		t.Fatalf("recordApprovalIssued: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.entries))
	}
	e := f.entries[0]
	if e.EventType != truthclient.EventApprovalIssued {
		t.Fatalf("event_type = %s, want %s", e.EventType, truthclient.EventApprovalIssued)
	}
	var p map[string]interface{}
	_ = json.Unmarshal(e.Payload, &p)
	if p["approval_id"] != "appr-truth-1" || p["quantity"] != 0.01 {
		t.Fatalf("payload not bound to approval: %v", p)
	}
}

// The write secret is sent as a bearer token when configured.
func TestRecordApprovalIssued_SendsBearerSecret(t *testing.T) {
	f := &fakeTruthCore{secret: "s3cret"}
	url := runFakeTruthCore(t, f)
	eng := &RiskEngine{redis: newFakeRedis(), truthClient: truthclient.New(url, "s3cret")}
	if err := eng.recordApprovalIssued(testApproval()); err != nil {
		t.Fatalf("recordApprovalIssued with secret: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.gotAuth) != 1 || f.gotAuth[0] != "Bearer s3cret" {
		t.Fatalf("bearer auth not sent: %v", f.gotAuth)
	}
}

// Wrong secret → 401 → approval fails closed.
func TestRecordApprovalIssued_WrongSecretFailsClosed(t *testing.T) {
	f := &fakeTruthCore{secret: "s3cret"}
	url := runFakeTruthCore(t, f)
	eng := &RiskEngine{redis: newFakeRedis(), truthClient: truthclient.New(url, "wrong")}
	if err := eng.recordApprovalIssued(testApproval()); err == nil {
		t.Fatal("wrong write secret did not fail the approval")
	}
}

// Tampering with the chain tip is detected by independent verification and
// engages the kill switch.
func TestTruthVerifyOnce_TipRewriteEngagesKill(t *testing.T) {
	f := &fakeTruthCore{}
	url := runFakeTruthCore(t, f)
	eng := &RiskEngine{redis: newFakeRedis(), truthClient: truthclient.New(url, "")}

	// Seed two clean entries through the client, then verify (full).
	for i := 0; i < 2; i++ {
		if err := eng.recordApprovalIssued(testApproval()); err != nil {
			t.Fatal(err)
		}
	}
	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("kill switch engaged on a clean chain")
	}
	if eng.truthTipID == 0 {
		t.Fatal("verified tip not recorded")
	}

	// Attacker rewrites the tip consistently (new payload + recomputed hash):
	// the pinned tip hash no longer matches.
	f.mu.Lock()
	last := &f.entries[len(f.entries)-1]
	last.Payload = json.RawMessage(`{"forged":true}`)
	last.Hash = tcHash(last.PrevHash, last.EventType, last.Payload)
	f.mu.Unlock()

	eng.truthVerifyOnce()
	if !eng.killSwitch.Load() {
		t.Fatal("rewritten tip did not engage the kill switch")
	}
}

// Corruption of an already-verified historical entry (leaving the tip
// intact) is caught by the periodic full re-verification.
func TestTruthVerifyOnce_HistoricalCorruptionEngagesKill(t *testing.T) {
	f := &fakeTruthCore{}
	url := runFakeTruthCore(t, f)
	eng := &RiskEngine{redis: newFakeRedis(), truthClient: truthclient.New(url, "")}

	for i := 0; i < 2; i++ {
		if err := eng.recordApprovalIssued(testApproval()); err != nil {
			t.Fatal(err)
		}
	}
	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("kill switch engaged on a clean chain")
	}

	// Corrupt an old entry's payload without touching the tip.
	f.mu.Lock()
	f.entries[0].Payload = json.RawMessage(`{"forged":true}`)
	f.mu.Unlock()

	// Force the hourly full re-verification.
	eng.truthMu.Lock()
	eng.truthLastFull = eng.truthLastFull.Add(-2 * time.Hour)
	eng.truthMu.Unlock()

	eng.truthVerifyOnce()
	if !eng.killSwitch.Load() {
		t.Fatal("historical corruption did not engage the kill switch")
	}
}

// An unreachable TruthCore alerts but does NOT kill: the append gate
// already blocks new approvals while the spine is down.
func TestTruthVerifyOnce_UnreachableDoesNotKill(t *testing.T) {
	eng := &RiskEngine{
		redis:       newFakeRedis(),
		truthClient: truthclient.New("http://127.0.0.1:1", ""),
	}
	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("unreachable truth-core engaged the kill switch")
	}
}
