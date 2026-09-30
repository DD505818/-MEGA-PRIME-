package approval

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"
)

func testKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func testApproval() *Approval {
	return &Approval{
		ApprovalID: "appr-1",
		SignalID:   "sig-1",
		StrategyID: "strat-1",
		Symbol:     "BTC/USD",
		Side:       "BUY",
		Quantity:   0.01,
		LimitPrice: 63000,
		StopPrice:  62000,
		Mode:       "paper",
	}
}

// Round-trip: sign then verify passes.
func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv := testKeypair(t)
	a := testApproval()
	a.Sign(priv, time.Now())
	if err := a.Verify(pub); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if a.IssuedAtMs == 0 || a.ExpiresAtMs <= a.IssuedAtMs {
		t.Fatal("timestamps not set correctly")
	}
}

// Tampering with ANY bound field breaks the signature.
func TestTamperBreaksSignature(t *testing.T) {
	fields := []func(a *Approval){
		func(a *Approval) { a.Quantity = 999 },
		func(a *Approval) { a.LimitPrice = 1 },
		func(a *Approval) { a.StopPrice = 1 },
		func(a *Approval) { a.Side = "SELL" },
		func(a *Approval) { a.Symbol = "ETH/USD" },
		func(a *Approval) { a.SignalID = "sig-evil" },
		func(a *Approval) { a.Mode = "live" },
		func(a *Approval) { a.ExpiresAtMs += 1000 },
	}
	for i, tamper := range fields {
		pub, priv := testKeypair(t)
		a := testApproval()
		a.Sign(priv, time.Now())
		tamper(a)
		if err := a.Verify(pub); err == nil {
			t.Fatalf("case %d: tampered approval verified", i)
		}
	}
}

// Wrong key fails.
func TestWrongKeyFails(t *testing.T) {
	pub2, _ := testKeypair(t)
	_, priv := testKeypair(t)
	a := testApproval()
	a.Sign(priv, time.Now())
	if err := a.Verify(pub2); err == nil {
		t.Fatal("approval signed by another key verified")
	}
}

// Expiry is enforced by the caller via Expired().
func TestExpiry(t *testing.T) {
	a := testApproval()
	now := time.Now()
	_, priv := testKeypair(t)
	a.Sign(priv, now)
	if a.Expired(now) {
		t.Fatal("fresh approval reports expired")
	}
	if !a.Expired(now.Add(ApprovalTTL + time.Second)) {
		t.Fatal("old approval does not report expired")
	}
}

// Canonical bytes are deterministic and exclude the signature.
func TestCanonicalDeterministic(t *testing.T) {
	_, priv := testKeypair(t)
	a1, a2 := testApproval(), testApproval()
	a1.Sign(priv, time.UnixMilli(123456789))
	a2.Sign(priv, time.UnixMilli(123456789))
	if string(a1.CanonicalBytes()) != string(a2.CanonicalBytes()) {
		t.Fatal("canonical bytes not deterministic")
	}
}

// Key parsing rejects garbage.
func TestParseKeysRejectGarbage(t *testing.T) {
	for _, bad := range []string{"", "not-base64!!!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := ParsePrivateKey(bad); err == nil {
			t.Fatalf("private key %q accepted", bad)
		}
		if _, err := ParsePublicKey(bad); err == nil {
			t.Fatalf("public key %q accepted", bad)
		}
	}
	_, priv := testKeypair(t)
	pub := priv.Public().(ed25519.PublicKey)
	seedB64 := base64.StdEncoding.EncodeToString(priv.Seed())
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	if _, err := ParsePrivateKey(seedB64); err != nil {
		t.Fatalf("valid private key rejected: %v", err)
	}
	if _, err := ParsePublicKey(pubB64); err != nil {
		t.Fatalf("valid public key rejected: %v", err)
	}
}

// Map round-trip through JSON (Kafka path).
func TestMapRoundTrip(t *testing.T) {
	pub, priv := testKeypair(t)
	a := testApproval()
	a.Sign(priv, time.Now())
	b, err := FromMap(a.ToMap())
	if err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}
	if err := b.Verify(pub); err != nil {
		t.Fatalf("verify after map round-trip failed: %v", err)
	}
	if b.Quantity != a.Quantity || b.LimitPrice != a.LimitPrice {
		t.Fatal("fields did not survive round-trip")
	}
}
