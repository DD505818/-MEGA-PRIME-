package main

// Tests for the durable TruthCore chain anchor (#1): the verified tip is
// persisted to Redis after every successful verification, and on boot
// the live chain must prove it still descends from that anchor. A
// rewritten/truncated chain — even one re-hashed consistently, which
// defeats from-genesis verification alone — fails closed.

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/omega-prime-delta/truthclient"
)

// seedChain appends n approval entries to the fake truth-core directly.
func seedChain(t *testing.T, f *fakeTruthCore, n int) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i < n; i++ {
		f.appendLocked(truthclient.EventApprovalIssued, json.RawMessage(`{"approval_id":"a1"}`))
	}
}

func anchoredEngine(t *testing.T, fr *fakeRedis, f *fakeTruthCore) *RiskEngine {
	t.Helper()
	return &RiskEngine{redis: fr, truthClient: truthclient.New(runFakeTruthCore(t, f), "")}
}

// rewriteChainConsistently forges entry payloads and recomputes the whole
// hash chain over them: the result is internally valid, so independent
// from-genesis verification alone would ACCEPT it. Only the external
// anchor can catch it.
func rewriteChainConsistently(f *fakeTruthCore, mutate func(i int) []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	prev := truthclient.GenesisPrevHash
	for i := range f.entries {
		if p := mutate(i); p != nil {
			f.entries[i].Payload = p
		}
		f.entries[i].PrevHash = prev
		f.entries[i].Hash = tcHash(prev, f.entries[i].EventType, f.entries[i].Payload)
		prev = f.entries[i].Hash
	}
}

// After a successful verification, the verified tip is persisted as the
// external anchor (with its epoch marker).
func TestTruthAnchor_PersistedAfterSuccessfulVerification(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	eng := anchoredEngine(t, fr, f)
	seedChain(t, f, 2)

	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("kill switch engaged on a clean chain")
	}
	ctx := context.Background()
	raw, err := fr.Get(ctx, truthAnchorKey).Result()
	if err != nil {
		t.Fatalf("anchor not persisted: %v", err)
	}
	var a truthAnchor
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatalf("anchor unparsable: %v", err)
	}
	f.mu.Lock()
	tipHash := f.entries[len(f.entries)-1].Hash
	f.mu.Unlock()
	if a.TipID != 2 || a.TipHash != tipHash {
		t.Fatalf("anchor = %+v, want tip 2 @ %s", a, tipHash)
	}
	if v, err := fr.Get(ctx, truthAnchorEpochKey).Result(); err != nil || v != "1" {
		t.Fatalf("epoch marker = %q, %v; want \"1\"", v, err)
	}
}

// Boot with a healthy, grown chain: continuity against the anchor is
// confirmed, no kill, and the in-process tip is pinned to the anchor.
func TestTruthAnchor_BootContinuityOK(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	eng1 := anchoredEngine(t, fr, f)
	seedChain(t, f, 2)
	eng1.truthVerifyOnce() // anchors tip 2
	seedChain(t, f, 1)     // chain grows to 3

	eng2 := anchoredEngine(t, fr, f) // restarted process
	eng2.restoreTruthAnchor()
	if eng2.killSwitch.Load() {
		t.Fatal("kill switch engaged on a continuous chain at boot")
	}
	if !eng2.truthAnchorResolved() {
		t.Fatal("anchor not resolved at boot")
	}
	if eng2.truthTipID != 2 {
		t.Fatalf("in-process tip = %d, want pinned anchor tip 2", eng2.truthTipID)
	}
}

// Boot after the tip was rewritten (consistently re-hashed): the anchor
// no longer matches the head — kill, and nothing is adopted.
func TestTruthAnchor_BootDetectsTipRewrite(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	eng1 := anchoredEngine(t, fr, f)
	seedChain(t, f, 2)
	eng1.truthVerifyOnce()

	rewriteChainConsistently(f, func(i int) []byte {
		if i == 1 {
			return json.RawMessage(`{"approval_id":"forged"}`)
		}
		return nil
	})

	eng2 := anchoredEngine(t, fr, f)
	eng2.restoreTruthAnchor()
	if !eng2.killSwitch.Load() {
		t.Fatal("rewritten tip did not engage the kill switch at boot")
	}
	if eng2.truthTipID != 0 {
		t.Fatal("forged chain tip was adopted")
	}
}

// Boot after history BELOW the anchor was rewritten and the whole chain
// re-hashed consistently: head height is above the anchor, so the
// anchored entry itself must be compared — it no longer matches.
func TestTruthAnchor_BootDetectsHistoryRewriteBelowAnchor(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	eng1 := anchoredEngine(t, fr, f)
	seedChain(t, f, 2)
	eng1.truthVerifyOnce() // anchor at tip 2
	seedChain(t, f, 1)     // honest growth to 3

	rewriteChainConsistently(f, func(i int) []byte {
		if i == 0 {
			return json.RawMessage(`{"approval_id":"forged-history"}`)
		}
		return nil
	})

	eng2 := anchoredEngine(t, fr, f)
	eng2.restoreTruthAnchor()
	if !eng2.killSwitch.Load() {
		t.Fatal("history rewrite below the anchor did not engage the kill switch")
	}
}

// Boot after truncation below the anchored height: kill.
func TestTruthAnchor_BootDetectsTruncation(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	eng1 := anchoredEngine(t, fr, f)
	seedChain(t, f, 2)
	eng1.truthVerifyOnce()

	f.mu.Lock()
	f.entries = f.entries[:1]
	f.mu.Unlock()

	eng2 := anchoredEngine(t, fr, f)
	eng2.restoreTruthAnchor()
	if !eng2.killSwitch.Load() {
		t.Fatal("truncated chain did not engage the kill switch at boot")
	}
}

// The anchor existed (epoch marker set) but is now missing, with a chain
// present: fail closed. The current head must NOT be silently adopted.
func TestTruthAnchor_MissingAfterFirstRunFailsClosed(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	seedChain(t, f, 2)
	if err := fr.Set(context.Background(), truthAnchorEpochKey, "1", 0).Err(); err != nil {
		t.Fatal(err)
	}

	eng := anchoredEngine(t, fr, f)
	eng.restoreTruthAnchor()
	if !eng.killSwitch.Load() {
		t.Fatal("missing anchor after first run did not fail closed")
	}
	if _, err := fr.Get(context.Background(), truthAnchorKey).Result(); err != redis.Nil {
		t.Fatalf("current head was silently adopted as anchor: err=%v", err)
	}
	// Verification stays blocked too.
	eng.truthVerifyOnce()
	if eng.truthTipID != 0 {
		t.Fatal("verification trusted a chain with a missing anchor")
	}
}

// Genuine first run (no anchor, no epoch): boot is fine, and the first
// successful verification establishes the anchor.
func TestTruthAnchor_FirstRunBootstrap(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	seedChain(t, f, 2)

	eng := anchoredEngine(t, fr, f)
	eng.restoreTruthAnchor()
	if eng.killSwitch.Load() {
		t.Fatal("first run engaged the kill switch")
	}
	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("first verification engaged the kill switch")
	}
	if _, err := fr.Get(context.Background(), truthAnchorKey).Result(); err != nil {
		t.Fatalf("anchor not established on first run: %v", err)
	}
}

// Empty chain and no anchor: nothing to anchor, nothing to kill.
func TestTruthAnchor_EmptyChainNoAnchorNoKill(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	eng := anchoredEngine(t, fr, f)
	eng.restoreTruthAnchor()
	if eng.killSwitch.Load() {
		t.Fatal("empty chain with no anchor engaged the kill switch")
	}
}

// Operator-armed re-anchor after anchor loss: no kill, and the anchor is
// re-established only after the full verification succeeds.
func TestTruthAnchor_MissingAnchorArmedReanchors(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	seedChain(t, f, 2)
	if err := fr.Set(context.Background(), truthAnchorEpochKey, "1", 0).Err(); err != nil {
		t.Fatal(err)
	}

	eng := anchoredEngine(t, fr, f)
	eng.truthAcceptCurrentTip = true
	eng.restoreTruthAnchor()
	if eng.killSwitch.Load() {
		t.Fatal("armed re-anchor engaged the kill switch")
	}
	if _, err := fr.Get(context.Background(), truthAnchorKey).Result(); err != redis.Nil {
		t.Fatal("anchor re-established before any verification")
	}
	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("re-anchor verification engaged the kill switch")
	}
	raw, err := fr.Get(context.Background(), truthAnchorKey).Result()
	if err != nil {
		t.Fatalf("anchor not re-established after full verification: %v", err)
	}
	var a truthAnchor
	if err := json.Unmarshal([]byte(raw), &a); err != nil || a.TipID != 2 {
		t.Fatalf("re-established anchor wrong: %q", raw)
	}
	if !eng.truthAnchorResolved() {
		t.Fatal("anchor not resolved after re-anchor")
	}
}

// A Redis failure while persisting the anchor must not kill trading: the
// in-process pinned tip still protects this run, and the (stale) anchor
// is re-validated at next boot.
func TestTruthAnchor_PersistFailureDoesNotKill(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	eng := anchoredEngine(t, fr, f)
	seedChain(t, f, 2)

	fr.setFailSet(errors.New("redis write boom"))
	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("anchor persist failure engaged the kill switch")
	}
	if eng.truthTipID != 2 {
		t.Fatalf("verified tip not recorded in-process: %d", eng.truthTipID)
	}
}

// Redis unreadable during the anchor check: verification defers (nothing
// trusted, nothing adopted) and does not kill on transport evidence.
func TestTruthAnchor_RedisUnreadableDefersClosed(t *testing.T) {
	f := &fakeTruthCore{}
	fr := newFakeRedis()
	seedChain(t, f, 2)
	fr.setFailGet(errors.New("redis read boom"))

	eng := anchoredEngine(t, fr, f)
	if got := eng.checkTruthAnchor(context.Background()); got != anchorUnresolved {
		t.Fatalf("checkTruthAnchor = %v, want anchorUnresolved", got)
	}
	eng.truthVerifyOnce()
	if eng.killSwitch.Load() {
		t.Fatal("unreadable Redis engaged the kill switch")
	}
	if eng.truthTipID != 0 {
		t.Fatal("verification trusted a chain while the anchor was unreadable")
	}
}
