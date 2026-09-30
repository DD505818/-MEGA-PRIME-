package truthclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"testing"
)

// fakeServer is an in-memory truth-core with the real hash algorithm.
type fakeServer struct {
	mu      sync.Mutex
	entries []Entry
}

func (f *fakeServer) append(eventType string, payload interface{}) Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	payloadBytes, _ := json.Marshal(payload)
	// Canonicalize like the server does (jsonb round-trip via Go map).
	var v interface{}
	_ = json.Unmarshal(payloadBytes, &v)
	payloadBytes, _ = json.Marshal(v)
	prev := GenesisPrevHash
	var id int64 = 1
	if len(f.entries) > 0 {
		prev = f.entries[len(f.entries)-1].Hash
		id = f.entries[len(f.entries)-1].ID + 1
	}
	e := Entry{
		ID: id, EntryID: fmt.Sprintf("e%d", id), EventType: eventType,
		Payload: payloadBytes, PrevHash: prev,
	}
	e.Hash = ComputeHash(e.PrevHash, e.EventType, e.Payload)
	f.entries = append(f.entries, e)
	return e
}

func (f *fakeServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	mux.HandleFunc("/append", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		e := f.append(body.EventType, body.Payload)
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
		json.NewEncoder(w).Encode(Head{ID: last.ID, EntryID: last.EntryID, Hash: last.Hash})
	})
	mux.HandleFunc("/entries", func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since_id"), 10, 64)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 500
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		var out []Entry
		for _, e := range f.entries {
			if e.ID > since {
				out = append(out, e)
				if len(out) >= limit {
					break
				}
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		if out == nil {
			out = []Entry{}
		}
		json.NewEncoder(w).Encode(out)
	})
	return mux
}

func testClient(t *testing.T, f *fakeServer) *Client {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return New(srv.URL, "")
}

func seedChain(f *fakeServer, n int) {
	for i := 0; i < n; i++ {
		f.append(EventApprovalIssued, map[string]interface{}{"i": i})
	}
}

func TestReady(t *testing.T) {
	f := &fakeServer{}
	c := testClient(t, f)
	if err := c.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
}

func TestVerifyIndependent_CleanChain(t *testing.T) {
	f := &fakeServer{}
	seedChain(f, 25)
	c := testClient(t, f)
	n, err := c.VerifyIndependent(context.Background())
	if err != nil {
		t.Fatalf("clean chain failed verification: %v", err)
	}
	if n != 25 {
		t.Fatalf("verified %d entries, want 25", n)
	}
}

func TestVerifyIndependent_DetectsTamperedPayload(t *testing.T) {
	f := &fakeServer{}
	seedChain(f, 10)
	// Tamper with a payload in place (as a DB-level attacker would).
	f.mu.Lock()
	f.entries[4].Payload = json.RawMessage(`{"i":"evil"}`)
	f.mu.Unlock()
	c := testClient(t, f)
	if _, err := c.VerifyIndependent(context.Background()); err == nil {
		t.Fatal("tampered payload not detected")
	}
}

func TestVerifyIndependent_DetectsRewrittenHash(t *testing.T) {
	f := &fakeServer{}
	seedChain(f, 10)
	f.mu.Lock()
	f.entries[6].Hash = f.entries[5].Hash // copy a wrong hash
	f.mu.Unlock()
	c := testClient(t, f)
	if _, err := c.VerifyIndependent(context.Background()); err == nil {
		t.Fatal("rewritten hash not detected")
	}
}

func TestVerifyIncremental_CleanAndNoop(t *testing.T) {
	f := &fakeServer{}
	seedChain(f, 5)
	c := testClient(t, f)
	n, err := c.VerifyIndependent(context.Background())
	if err != nil || n != 5 {
		t.Fatalf("full verify: n=%d err=%v", n, err)
	}
	head, _ := c.Head(context.Background())
	// No new entries → noop, tip unchanged.
	tipID, tipHash, count, err := c.VerifyIncremental(context.Background(), head.ID, head.Hash)
	if err != nil || count != 0 || tipID != head.ID || tipHash != head.Hash {
		t.Fatalf("noop incremental: %v %d", err, count)
	}
	// New entries → verified incrementally.
	seedChain(f, 3)
	tipID, tipHash, count, err = c.VerifyIncremental(context.Background(), head.ID, head.Hash)
	if err != nil || count != 3 {
		t.Fatalf("incremental: count=%d err=%v", count, err)
	}
	newHead, _ := c.Head(context.Background())
	if tipID != newHead.ID || tipHash != newHead.Hash {
		t.Fatal("tip not advanced to head")
	}
}

func TestVerifyIncremental_DetectsTailTruncation(t *testing.T) {
	f := &fakeServer{}
	seedChain(f, 10)
	c := testClient(t, f)
	if _, err := c.VerifyIndependent(context.Background()); err != nil {
		t.Fatal(err)
	}
	head, _ := c.Head(context.Background())
	// Attacker deletes the last 3 rows.
	f.mu.Lock()
	f.entries = f.entries[:7]
	f.mu.Unlock()
	if _, _, _, err := c.VerifyIncremental(context.Background(), head.ID, head.Hash); err == nil {
		t.Fatal("tail truncation not detected")
	}
}

func TestVerifyIncremental_DetectsFrontTruncation(t *testing.T) {
	f := &fakeServer{}
	seedChain(f, 10)
	c := testClient(t, f)
	if _, err := c.VerifyIndependent(context.Background()); err != nil {
		t.Fatal(err)
	}
	head, _ := c.Head(context.Background())
	// Attacker deletes the first 3 rows: later links still verify, head
	// unchanged — only the genesis anchor catches it.
	f.mu.Lock()
	f.entries = f.entries[3:]
	f.mu.Unlock()
	if _, _, _, err := c.VerifyIncremental(context.Background(), head.ID, head.Hash); err == nil {
		t.Fatal("front truncation not detected")
	}
}

func TestVerifyIncremental_DetectsFork(t *testing.T) {
	f := &fakeServer{}
	seedChain(f, 8)
	c := testClient(t, f)
	if _, err := c.VerifyIndependent(context.Background()); err != nil {
		t.Fatal(err)
	}
	head, _ := c.Head(context.Background())
	// Attacker rewrites the tip row (new hash) without appending.
	f.mu.Lock()
	last := &f.entries[len(f.entries)-1]
	last.Payload = json.RawMessage(`{"forged":true}`)
	last.Hash = ComputeHash(last.PrevHash, last.EventType, last.Payload)
	f.mu.Unlock()
	if _, _, _, err := c.VerifyIncremental(context.Background(), head.ID, head.Hash); err == nil {
		t.Fatal("tip rewrite not detected")
	}
}

func TestAppend_RoundTrip(t *testing.T) {
	f := &fakeServer{}
	c := testClient(t, f)
	e, err := c.Append(context.Background(), EventFill, map[string]interface{}{"order_id": "o1"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if e.EventType != EventFill {
		t.Fatalf("event type = %s", e.EventType)
	}
	if n, err := c.VerifyIndependent(context.Background()); err != nil || n != 1 {
		t.Fatalf("verify after append: n=%d err=%v", n, err)
	}
}
