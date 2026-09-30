// Package truthclient is the shared TruthCore client for the ΩMEGA PRIME Δ
// services. It speaks to the truth-core HTTP API and — critically — can
// verify the hash chain INDEPENDENTLY: it fetches raw entries (prev_hash,
// hash, event_type, canonical payload bytes) and recomputes every link
// locally, without trusting the server's /verify endpoint.
//
// Canonical trade-lifecycle event types (immutable lineage):
//
//	aegis.approval_issued   — risk-service, after signing an approval
//	vulture.order_submitted — execution-service, after approval verification,
//	                          before routing (audit-or-no-trade gate)
//	vulture.fill            — execution-service, after a fill
//	vulture.order_refused   — execution-service, when submission is refused
//
// Lineage: approval_issued --approval_id--> order_submitted --order_id--> fill.
// Every event carries its parent's ID so the full lineage is reconstructible
// from the log alone.
package truthclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Canonical event types for the trade lifecycle.
const (
	EventApprovalIssued = "aegis.approval_issued"
	EventOrderSubmitted = "vulture.order_submitted"
	EventFill           = "vulture.fill"
	EventOrderRefused   = "vulture.order_refused"
)

// GenesisPrevHash is the prev_hash of the first entry in a chain.
const GenesisPrevHash = "genesis"

// Entry is one audit-log row as returned by the server.
type Entry struct {
	ID        int64           `json:"id"`
	EntryID   string          `json:"entry_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	PrevHash  string          `json:"prev_hash"`
	Hash      string          `json:"hash"`
}

// Head is the chain tip.
type Head struct {
	ID      int64  `json:"id"`
	EntryID string `json:"entry_id"`
	Hash    string `json:"hash"`
}

// ComputeHash recomputes the entry hash exactly as the server does:
// SHA-256(prev_hash || 0x00 || event_type || 0x00 || canonical_payload).
func ComputeHash(prevHash, eventType string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte{0})
	h.Write([]byte(eventType))
	h.Write([]byte{0})
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// Client talks to a truth-core instance.
type Client struct {
	base   string
	secret string // optional bearer token for /append
	http   *http.Client
}

// New creates a client. base is e.g. "http://truth-core:8084". If secret is
// non-empty it is sent as a bearer token on writes.
func New(base, secret string) *Client {
	return &Client{
		base:   base,
		secret: secret,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body interface{}) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("truthclient: marshal: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	u := c.base + path
	if query != nil {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, fmt.Errorf("truthclient: request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" && method != http.MethodGet {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("truthclient: %s %s: %w", method, path, err)
	}
	return resp, nil
}

func decodeJSON(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("truthclient: status %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Append writes one event to the hash chain. Used at audit-or-no-trade gates:
// the caller treats an error as a refusal to proceed.
func (c *Client) Append(ctx context.Context, eventType string, payload interface{}) (*Entry, error) {
	resp, err := c.do(ctx, http.MethodPost, "/append", nil, map[string]interface{}{
		"event_type": eventType,
		"payload":    payload,
	})
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := decodeJSON(resp, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Head returns the chain tip.
func (c *Client) Head(ctx context.Context) (*Head, error) {
	resp, err := c.do(ctx, http.MethodGet, "/head", nil, nil)
	if err != nil {
		return nil, err
	}
	var h Head
	if err := decodeJSON(resp, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// Entries returns entries with id > sinceID, oldest first, up to limit.
func (c *Client) Entries(ctx context.Context, sinceID int64, limit int) ([]Entry, error) {
	q := url.Values{}
	q.Set("since_id", strconv.FormatInt(sinceID, 10))
	q.Set("limit", strconv.Itoa(limit))
	resp, err := c.do(ctx, http.MethodGet, "/entries", q, nil)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := decodeJSON(resp, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []Entry{}
	}
	return entries, nil
}

// IntegrityError marks a verification failure caused by chain-integrity
// evidence (broken link, hash mismatch, truncation, gap, fork, rewrite,
// broken genesis anchor) as opposed to transport failures. Callers that
// enforce audit-or-no-trade use errors.As to decide fail-closed responses.
type IntegrityError struct{ Msg string }

func (e *IntegrityError) Error() string { return "integrity: " + e.Msg }

func integrityErr(format string, args ...interface{}) *IntegrityError {
	return &IntegrityError{Msg: fmt.Sprintf(format, args...)}
}

// verifyBatch checks one contiguous batch: the first entry's prev_hash must
// equal expectedPrev, every link must recompute, and ids must be contiguous.
func verifyBatch(entries []Entry, expectedPrev string) (string, error) {
	prev := expectedPrev
	for i, e := range entries {
		if i > 0 && e.ID != entries[i-1].ID+1 {
			return "", integrityErr("id gap: entry %d follows %d", e.ID, entries[i-1].ID)
		}
		if e.PrevHash != prev {
			return "", integrityErr("chain broken at id=%d: expected prev_hash %.12s, got %.12s",
				e.ID, prev, e.PrevHash)
		}
		if got := ComputeHash(e.PrevHash, e.EventType, e.Payload); got != e.Hash {
			return "", integrityErr("hash mismatch at id=%d: stored %.12s, computed %.12s",
				e.ID, e.Hash, got)
		}
		prev = e.Hash
	}
	return prev, nil
}

// VerifyIndependent recomputes the ENTIRE chain locally from genesis,
// trusting nothing but the raw entry bytes. Returns the verified count.
func (c *Client) VerifyIndependent(ctx context.Context) (int, error) {
	const page = 500
	var (
		count int
		prev  = GenesisPrevHash
		since int64
	)
	for {
		entries, err := c.Entries(ctx, since, page)
		if err != nil {
			return count, err
		}
		if len(entries) == 0 {
			return count, nil
		}
		// Contiguity with the previous page.
		if count > 0 && entries[0].ID != since+1 {
			return count, integrityErr("page gap: expected id %d, got %d", since+1, entries[0].ID)
		}
		newPrev, err := verifyBatch(entries, prev)
		if err != nil {
			return count, err
		}
		prev = newPrev
		count += len(entries)
		since = entries[len(entries)-1].ID
	}
}

// VerifyIncremental verifies entries after a previously verified tip
// (tipID, tipHash) and anchors against the current server head. It detects
// truncation (head moved backwards), forks (head hash mismatch after a clean
// batch), and gaps. Returns the new verified tip.
func (c *Client) VerifyIncremental(ctx context.Context, tipID int64, tipHash string) (int64, string, int, error) {
	head, err := c.Head(ctx)
	if err != nil {
		return tipID, tipHash, 0, fmt.Errorf("head: %w", err)
	}
	if head.ID < tipID {
		return tipID, tipHash, 0, integrityErr("chain truncation detected: head id %d < verified tip %d",
			head.ID, tipID)
	}
	// Genesis anchor: deleting the oldest rows keeps all later links valid,
	// so front-truncation is invisible to link checks. The first row must
	// always be id=1 chained from genesis.
	oldest, err := c.Entries(ctx, 0, 1)
	if err != nil {
		return tipID, tipHash, 0, fmt.Errorf("genesis anchor: %w", err)
	}
	if len(oldest) == 0 || oldest[0].ID != 1 || oldest[0].PrevHash != GenesisPrevHash {
		return tipID, tipHash, 0, integrityErr("genesis anchor broken: oldest entry is not id=1 from genesis (front-truncation?)")
	}
	if head.ID == tipID {
		if head.Hash != tipHash {
			return tipID, tipHash, 0, integrityErr("tip hash changed without new entries: possible rewrite")
		}
		return tipID, tipHash, 0, nil // nothing new
	}
	const page = 500
	var (
		count int
		prev  = tipHash
		since = tipID
	)
	for {
		entries, err := c.Entries(ctx, since, page)
		if err != nil {
			return tipID, tipHash, count, err
		}
		if len(entries) == 0 {
			break
		}
		if entries[0].ID != since+1 {
			return tipID, tipHash, count, integrityErr("gap after id %d: next is %d", since, entries[0].ID)
		}
		newPrev, err := verifyBatch(entries, prev)
		if err != nil {
			return tipID, tipHash, count, err
		}
		prev = newPrev
		count += len(entries)
		since = entries[len(entries)-1].ID
		if since >= head.ID {
			break
		}
	}
	if since != head.ID || prev != head.Hash {
		return tipID, tipHash, count, integrityErr(
			"head mismatch after clean batch: reached id %d hash %.12s, server head id %d hash %.12s",
			since, prev, head.ID, head.Hash)
	}
	return head.ID, head.Hash, count, nil
}
