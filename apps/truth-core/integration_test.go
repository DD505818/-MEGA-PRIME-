package main

// Live-PostgreSQL integration tests for TruthCore.
//
// These tests exercise the real HTTP handlers, the real truthclient, and a
// real PostgreSQL server — the layers the unit suites replace with fakes.
// They are gated on TRUTHCORE_TEST_DSN: when the variable is unset every
// test here SKIPs, so plain `go test ./...` stays green with no database.
//
// To run them:
//
//	initdb a scratch cluster, create a database, then:
//	TRUTHCORE_TEST_DSN='postgres://postgres@127.0.0.1:54399/truthcore_test?sslmode=disable' \
//	    go test -run Integration -v ./...
//
// Each test resets the audit table (DROP + initSQL) so it starts from
// genesis. The DSN should point at a throwaway database.
//
// KNOWN DEFECT pinned by these tests (found 2026-09-30 against live
// PostgreSQL 16.4; invisible to every mock-based suite): the server hashes
// the PostgreSQL jsonb text form of the payload (e.g. {"i": -1} — spaced,
// key-sorted), but /entries and /recent transport payloads through Go's
// JSON encoder, which compacts json.RawMessage ({"i":-1}). truthclient
// therefore recomputes a different hash for every entry, so
// VerifyIndependent can NEVER pass against a real database, even on a
// healthy chain. The server's own VerifyChain still passes because it
// reads the spaced bytes straight from pgx. Until the production hash
// input is made transport-stable, the tests below that assert
// VerifyIndependent success are EXPECTED TO FAIL against a live DB; they
// encode the desired behavior on purpose. Everything else they assert
// (auth, triggers, round-trip, lineage, duplicate behavior, concurrent
// chain integrity) passes and is pinned as the current contract.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/omega-prime-delta/truthclient"
)

const testWriteSecret = "integration-test-secret"

// liveServer is a truth-core HTTP server (real handlers, real mux shape as
// main) backed by a real PostgreSQL database, plus an authenticated
// truthclient pointed at it.
type liveServer struct {
	tc     *TruthCore
	pool   *pgxpool.Pool
	srv    *httptest.Server
	client *truthclient.Client
}

func liveDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TRUTHCORE_TEST_DSN")
	if dsn == "" {
		t.Skip("TRUTHCORE_TEST_DSN not set — skipping live PostgreSQL integration test")
	}
	return dsn
}

// resetChain drops the audit table and re-applies the production initSQL so
// the next test starts from genesis.
func resetChain(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS audit_log CASCADE`); err != nil {
		t.Fatalf("reset chain: drop audit_log: %v", err)
	}
	if _, err := pool.Exec(ctx, initSQL); err != nil {
		t.Fatalf("reset chain: init schema: %v", err)
	}
}

func newLiveServer(t *testing.T) *liveServer {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), liveDSN(t))
	if err != nil {
		t.Fatalf("connect to live postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping live postgres: %v", err)
	}
	resetChain(t, pool)

	tc := &TruthCore{db: pool}
	mux := http.NewServeMux()
	// Same wiring as main(): /append behind the bearer-secret guard.
	mux.HandleFunc("/append", requireWriteAuth(testWriteSecret, tc.appendHandler))
	mux.HandleFunc("/verify", tc.verifyHandler)
	mux.HandleFunc("/head", tc.headHandler)
	mux.HandleFunc("/entries", tc.entriesHandler)
	mux.HandleFunc("/lineage", tc.lineageHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &liveServer{
		tc:     tc,
		pool:   pool,
		srv:    srv,
		client: truthclient.New(srv.URL, testWriteSecret),
	}
}

func postAppendRaw(t *testing.T, url, authHeader string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/append", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /append: %v", err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp.StatusCode, buf.Bytes()
}

// TestIntegrationAppendAuth: /append must reject missing and wrong bearer
// secrets and accept the configured TRUTHCORE_WRITE_SECRET, end to end
// through the real HTTP guard and database.
func TestIntegrationAppendAuth(t *testing.T) {
	ls := newLiveServer(t)
	body := []byte(`{"event_type":"test.auth","payload":{"k":"v"}}`)

	if code, _ := postAppendRaw(t, ls.srv.URL, "", body); code != http.StatusUnauthorized {
		t.Errorf("missing secret: got status %d, want 401", code)
	}
	if code, _ := postAppendRaw(t, ls.srv.URL, "Bearer wrong-secret", body); code != http.StatusUnauthorized {
		t.Errorf("wrong secret: got status %d, want 401", code)
	}

	ctx := context.Background()
	bad := truthclient.New(ls.srv.URL, "wrong-secret")
	if _, err := bad.Append(ctx, "test.auth", map[string]string{"k": "v"}); err == nil {
		t.Error("truthclient with wrong secret: append succeeded, want error")
	}

	code, respBody := postAppendRaw(t, ls.srv.URL, "Bearer "+testWriteSecret, body)
	if code != http.StatusOK {
		t.Fatalf("correct secret: got status %d (%s), want 200", code, respBody)
	}
	var e AuditEntry
	if err := json.Unmarshal(respBody, &e); err != nil {
		t.Fatalf("decode append response: %v", err)
	}
	if e.PrevHash != "genesis" {
		t.Errorf("first entry prev_hash = %q, want genesis", e.PrevHash)
	}

	// The two rejected appends above must not have persisted anything:
	// exactly one entry (the authorized one) exists.
	entries, err := ls.client.Entries(ctx, 0, 10)
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("entries after auth attempts = %d, want 1 (rejected writes must not persist)", len(entries))
	}
}

// TestIntegrationHeadEntriesLineage: appended lifecycle events round-trip
// through /head, /entries, and /lineage, and the chain verifies
// independently via truthclient recomputation.
func TestIntegrationHeadEntriesLineage(t *testing.T) {
	ls := newLiveServer(t)
	ctx := context.Background()

	approval := map[string]interface{}{
		"approval_id": "appr-int-1", "symbol": "BTC/USD", "side": "buy",
		"mode": "paper", "quantity": 0.5,
	}
	order := map[string]interface{}{
		"approval_id": "appr-int-1", "order_id": "ord-int-1", "symbol": "BTC/USD",
		"side": "buy", "mode": "paper", "quantity": 0.5,
	}
	fill := map[string]interface{}{
		"approval_id": "appr-int-1", "order_id": "ord-int-1", "quantity": 0.5, "price": 100.0,
	}
	e1, err := ls.client.Append(ctx, eventApprovalIssued, approval)
	if err != nil {
		t.Fatalf("append approval: %v", err)
	}
	e2, err := ls.client.Append(ctx, eventOrderSubmitted, order)
	if err != nil {
		t.Fatalf("append order: %v", err)
	}
	e3, err := ls.client.Append(ctx, eventFill, fill)
	if err != nil {
		t.Fatalf("append fill: %v", err)
	}
	if e1.PrevHash != "genesis" || e2.PrevHash != e1.Hash || e3.PrevHash != e2.Hash {
		t.Fatalf("hash links wrong: e1.prev=%q e2.prev=%q e3.prev=%q", e1.PrevHash, e2.PrevHash, e3.PrevHash)
	}

	head, err := ls.client.Head(ctx)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if head.ID != e3.ID || head.Hash != e3.Hash || head.EntryID != e3.EntryID {
		t.Errorf("head = %+v, want tip of third entry (id %d)", head, e3.ID)
	}

	entries, err := ls.client.Entries(ctx, 0, 10)
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	for i, want := range []string{eventApprovalIssued, eventOrderSubmitted, eventFill} {
		if entries[i].EventType != want {
			t.Errorf("entries[%d].EventType = %q, want %q", i, entries[i].EventType, want)
		}
	}
	var gotApproval map[string]interface{}
	if err := json.Unmarshal(entries[0].Payload, &gotApproval); err != nil {
		t.Fatalf("decode stored payload: %v", err)
	}
	if gotApproval["approval_id"] != "appr-int-1" {
		t.Errorf("stored payload approval_id = %v, want appr-int-1 (payload round-trip)", gotApproval["approval_id"])
	}

	// Lineage by approval_id and by order_id, via the raw HTTP API.
	for _, query := range []string{"approval_id=appr-int-1", "order_id=ord-int-1"} {
		resp, err := http.Get(ls.srv.URL + "/lineage?" + query)
		if err != nil {
			t.Fatalf("lineage %s: %v", query, err)
		}
		var lineage []AuditEntry
		err = json.NewDecoder(resp.Body).Decode(&lineage)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("lineage %s decode: %v", query, err)
		}
		if len(lineage) != 3 {
			t.Errorf("lineage?%s returned %d entries, want 3", query, len(lineage))
			continue
		}
		if lineage[0].EventType != eventApprovalIssued || lineage[2].EventType != eventFill {
			t.Errorf("lineage?%s order wrong: %s ... %s", query, lineage[0].EventType, lineage[2].EventType)
		}
	}

	// The server's own /verify (raw pgx reads, same spaced jsonb bytes it
	// hashed) reports the chain valid...
	resp, err := http.Get(ls.srv.URL + "/verify")
	if err != nil {
		t.Fatalf("GET /verify: %v", err)
	}
	var verifyResp struct {
		Valid           bool `json:"valid"`
		VerifiedEntries int  `json:"verified_entries"`
	}
	err = json.NewDecoder(resp.Body).Decode(&verifyResp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("decode /verify: %v", err)
	}
	if !verifyResp.Valid || verifyResp.VerifiedEntries != 3 {
		t.Errorf("/verify = %+v, want valid with 3 entries", verifyResp)
	}

	// ...while independent truthclient verification recomputes over the
	// HTTP-transported (Go-compacted) payload bytes. As of 8fd4b36 this
	// FAILS on a healthy chain — see the KNOWN DEFECT note at the top of
	// this file. This assertion encodes the desired end-to-end behavior
	// and runs last so every other assertion above executes first.
	n, err := ls.client.VerifyIndependent(ctx)
	if err != nil {
		t.Fatalf("independent verification: %v", err)
	}
	if n != 3 {
		t.Errorf("verified entries = %d, want 3", n)
	}
}

// TestIntegrationAppendOnlyTriggers: the live database itself must reject
// UPDATE and DELETE against audit_log via the production trigger — not
// merely trust the application never to issue them.
func TestIntegrationAppendOnlyTriggers(t *testing.T) {
	ls := newLiveServer(t)
	ctx := context.Background()

	if _, err := ls.client.Append(ctx, "test.trigger", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	if _, err := ls.pool.Exec(ctx, `UPDATE audit_log SET event_type = 'tampered' WHERE id = 1`); err == nil {
		t.Error("UPDATE on audit_log succeeded — append-only trigger did not fire")
	} else if !strings.Contains(err.Error(), "append-only") {
		t.Errorf("UPDATE error = %v, want append-only trigger rejection", err)
	}
	if _, err := ls.pool.Exec(ctx, `DELETE FROM audit_log WHERE id = 1`); err == nil {
		t.Error("DELETE on audit_log succeeded — append-only trigger did not fire")
	} else if !strings.Contains(err.Error(), "append-only") {
		t.Errorf("DELETE error = %v, want append-only trigger rejection", err)
	}

	n, err := ls.client.VerifyIndependent(ctx)
	if err != nil || n != 1 {
		t.Errorf("after rejected mutations: verified=%d err=%v, want 1 entry still valid", n, err)
	}
}

// TestIntegrationDuplicateAppendRetry pins the CURRENT /append retry
// behavior: there is no idempotency key, so retrying an identical append
// creates a second, distinct entry. Double-spend protection lives at the
// VULTURE/Gate 11 layer, not in the log; this test exists so the gap is
// documented by an assertion rather than discovered later. The chain must
// still extend (not fork) across the duplicate.
func TestIntegrationDuplicateAppendRetry(t *testing.T) {
	ls := newLiveServer(t)
	ctx := context.Background()

	payload := map[string]interface{}{"approval_id": "appr-dup-1", "symbol": "BTC/USD"}
	e1, err := ls.client.Append(ctx, eventApprovalIssued, payload)
	if err != nil {
		t.Fatalf("first append: %v", err)
	}
	e2, err := ls.client.Append(ctx, eventApprovalIssued, payload)
	if err != nil {
		t.Fatalf("retry append: %v (current behavior accepts retries as new entries)", err)
	}
	if e1.EntryID == e2.EntryID {
		t.Error("duplicate append reused entry_id — ids must be distinct")
	}
	if e2.PrevHash != e1.Hash {
		t.Error("duplicate append did not extend the chain (prev_hash != first entry hash)")
	}

	entries, err := ls.client.Entries(ctx, 0, 10)
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2 (identical retry persisted as a distinct entry — no idempotency)", len(entries))
	}
	if n, err := ls.client.VerifyIndependent(ctx); err != nil || n != 2 {
		t.Errorf("verified = %d, err = %v; want 2 valid entries", n, err)
	}
}

// TestIntegrationConcurrentAppends fires 32 simultaneous appends at the
// live server. Correctness contract: the chain must never fork or corrupt —
// every accepted append appears exactly once, entry count equals the number
// of accepted appends, and the whole chain recomputes via truthclient. The
// number of accepted vs rejected appends is logged prominently; rejected
// appends must fail loudly (explicit errors), never silently.
func TestIntegrationConcurrentAppends(t *testing.T) {
	ls := newLiveServer(t)
	ctx := context.Background()

	// Seed one entry so the burst is not racing on genesis.
	if _, err := ls.client.Append(ctx, "test.seed", map[string]int{"i": -1}); err != nil {
		t.Fatalf("seed append: %v", err)
	}

	const n = 32
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = ls.client.Append(ctx, "test.concurrent",
				map[string]interface{}{"worker": i, "note": fmt.Sprintf("burst-%d", i)})
		}(i)
	}
	close(start)
	wg.Wait()

	accepted := 0
	for i, err := range errs {
		if err == nil {
			accepted++
		} else {
			t.Logf("append %d rejected: %v", i, err)
		}
	}
	t.Logf("concurrent appends: %d/%d accepted, %d rejected", accepted, n, n-accepted)
	if accepted == 0 {
		t.Fatal("no concurrent append was accepted")
	}

	entries, err := ls.client.Entries(ctx, 0, 1000)
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	if len(entries) != accepted+1 {
		t.Errorf("entries = %d, want %d (seed + accepted appends; no silent loss, no duplicates)",
			len(entries), accepted+1)
	}

	// No fork: every prev_hash (except genesis) names the previous entry's
	// hash, and no prev_hash is shared by two entries.
	prevSeen := map[string]int64{}
	for i, e := range entries {
		if i == 0 {
			if e.PrevHash != "genesis" {
				t.Errorf("entry %d prev_hash = %q, want genesis", e.ID, e.PrevHash)
			}
		} else if e.PrevHash != entries[i-1].Hash {
			t.Errorf("entry %d prev_hash does not name entry %d's hash (fork or gap)", e.ID, entries[i-1].ID)
		}
		if other, dup := prevSeen[e.PrevHash]; dup {
			t.Errorf("prev_hash %q shared by entries %d and %d — FORKED CHAIN", e.PrevHash, other, e.ID)
		}
		prevSeen[e.PrevHash] = e.ID
	}

	count, err := ls.client.VerifyIndependent(ctx)
	if err != nil {
		t.Fatalf("independent verification after burst: %v", err)
	}
	if count != len(entries) {
		t.Errorf("verified %d entries, want %d", count, len(entries))
	}
	if vcount, verr := ls.tc.VerifyChain(ctx); verr != nil || vcount != len(entries) {
		t.Errorf("server VerifyChain: %d entries, err %v; want %d valid", vcount, verr, len(entries))
	}
}
