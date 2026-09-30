// Package approval implements the AEGIS→VULTURE authority boundary.
//
// Every order the execution service (VULTURE) may submit must carry a
// signed, single-use, expiring approval issued by the risk service (AEGIS).
// The approval binds every execution-critical field; VULTURE verifies the
// Ed25519 signature, the expiry, the field binding against the signal, and
// claims single-use atomically in Redis before any order state or routing.
//
// Fail-closed rules:
//   - No approval, bad signature, expired approval, or field mismatch → no trade.
//   - An approval ID that was already claimed → replay → no trade.
//   - Missing/unparseable keys at startup → the service refuses to start.
package approval

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ApprovalTTL bounds how long an issued approval is valid. Approvals must be
// fresh: this closes the approve→submit race window alongside the
// kill-switch pre-submit check.
const ApprovalTTL = 60 * time.Second

// ClaimKeyTTL is how long the single-use claim record lives in Redis. It
// comfortably exceeds ApprovalTTL, so a replay can never outlive the claim;
// expiry of the approval itself is the backstop after the claim lapses.
const ClaimKeyTTL = 10 * time.Minute

// GatesVersion identifies the risk policy that issued the approval.
const GatesVersion = "aegis-14-gate/v1"

// Approval binds every execution-critical field of one approved signal.
// Signature covers CanonicalBytes(); it is NOT part of the signed payload.
type Approval struct {
	ApprovalID   string  `json:"approval_id"`
	SignalID     string  `json:"signal_id"`
	StrategyID   string  `json:"strategy_id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	Quantity     float64 `json:"quantity"`
	LimitPrice   float64 `json:"limit_price"`
	StopPrice    float64 `json:"stop_price"`
	Mode         string  `json:"mode"`
	IssuedAtMs   int64   `json:"issued_at_ms"`
	ExpiresAtMs  int64   `json:"expires_at_ms"`
	GatesVersion string  `json:"gates_version"`
	Signature    string  `json:"signature"` // base64 Ed25519 over CanonicalBytes()
}

// canonicalForm is the signed payload: every field except Signature, in
// fixed declaration order (Go marshals struct fields in order, so the bytes
// are deterministic).
type canonicalForm struct {
	ApprovalID   string  `json:"approval_id"`
	SignalID     string  `json:"signal_id"`
	StrategyID   string  `json:"strategy_id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	Quantity     float64 `json:"quantity"`
	LimitPrice   float64 `json:"limit_price"`
	StopPrice    float64 `json:"stop_price"`
	Mode         string  `json:"mode"`
	IssuedAtMs   int64   `json:"issued_at_ms"`
	ExpiresAtMs  int64   `json:"expires_at_ms"`
	GatesVersion string  `json:"gates_version"`
}

// CanonicalBytes returns the deterministic signed payload.
func (a *Approval) CanonicalBytes() []byte {
	c := canonicalForm{
		ApprovalID:   a.ApprovalID,
		SignalID:     a.SignalID,
		StrategyID:   a.StrategyID,
		Symbol:       a.Symbol,
		Side:         a.Side,
		Quantity:     a.Quantity,
		LimitPrice:   a.LimitPrice,
		StopPrice:    a.StopPrice,
		Mode:         a.Mode,
		IssuedAtMs:   a.IssuedAtMs,
		ExpiresAtMs:  a.ExpiresAtMs,
		GatesVersion: a.GatesVersion,
	}
	b, _ := json.Marshal(c) // struct marshal cannot fail
	return b
}

// Sign attaches IssuedAtMs/ExpiresAtMs, then signs the canonical bytes.
func (a *Approval) Sign(priv ed25519.PrivateKey, now time.Time) {
	a.IssuedAtMs = now.UnixMilli()
	a.ExpiresAtMs = now.Add(ApprovalTTL).UnixMilli()
	if a.GatesVersion == "" {
		a.GatesVersion = GatesVersion
	}
	sig := ed25519.Sign(priv, a.CanonicalBytes())
	a.Signature = base64.StdEncoding.EncodeToString(sig)
}

// Verify checks the signature over the canonical bytes. It does NOT check
// expiry or field binding — the caller does that against its own clock and
// the signal it received.
func (a *Approval) Verify(pub ed25519.PublicKey) error {
	if a.Signature == "" {
		return errors.New("approval: missing signature")
	}
	sig, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil {
		return fmt.Errorf("approval: malformed signature: %w", err)
	}
	if !ed25519.Verify(pub, a.CanonicalBytes(), sig) {
		return errors.New("approval: signature verification failed")
	}
	return nil
}

// Expired reports whether the approval is past its validity window.
func (a *Approval) Expired(now time.Time) bool {
	return now.UnixMilli() >= a.ExpiresAtMs
}

// ClaimKey is the Redis key used for the atomic single-use claim.
func (a *Approval) ClaimKey() string {
	return "aegis:approval:" + a.ApprovalID
}

// ParsePrivateKey decodes a base64 32-byte Ed25519 seed into a private key.
// Env var: AEGIS_APPROVAL_PRIVKEY (risk-service only).
func ParsePrivateKey(b64 string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("approval: private key is not valid base64: %w", err)
	}
	if len(raw) != ed25519.SeedSize {
		return nil, fmt.Errorf("approval: private key must decode to %d bytes, got %d", ed25519.SeedSize, len(raw))
	}
	return ed25519.NewKeyFromSeed(raw), nil
}

// ParsePublicKey decodes a base64 32-byte Ed25519 public key.
// Env var: AEGIS_APPROVAL_PUBKEY (execution-service).
func ParsePublicKey(b64 string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("approval: public key is not valid base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("approval: public key must decode to %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// GenerateKeypair creates a fresh Ed25519 keypair for the approval boundary.
func GenerateKeypair() (pub ed25519.PublicKey, priv ed25519.PrivateKey, err error) {
	return ed25519.GenerateKey(rand.Reader)
}

// FromMap reconstructs an Approval from a decoded JSON object (e.g. the
// nested "aegis_approval" field of a signal consumed from Kafka).
func FromMap(m map[string]interface{}) (*Approval, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("approval: cannot re-encode approval map: %w", err)
	}
	var a Approval
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("approval: cannot decode approval: %w", err)
	}
	if a.ApprovalID == "" {
		return nil, errors.New("approval: missing approval_id")
	}
	return &a, nil
}

// ToMap serializes the approval for embedding in a signal map.
func (a *Approval) ToMap() map[string]interface{} {
	b, _ := json.Marshal(a)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}
