// Truth-Core — append-only, SHA-256 hash-chained audit log.
//
// Every INSERT reads the previous row's hash under SELECT ... FOR UPDATE,
// computes SHA-256(prev_hash || event_type || record_json), and commits atomically.
// Any gap or reordering is detected on read via VerifyChain().
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/omega-prime-delta/modelock"
)

const initSQL = `
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    entry_id    UUID        NOT NULL UNIQUE,
    event_type  TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    prev_hash   TEXT        NOT NULL,
    hash        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_log_event ON audit_log(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
-- Immutability: the audit log is append-only at the SQL level too. Even a
-- superuser connection cannot UPDATE or DELETE rows; tampering must break
-- the hash chain to be invisible, and the chain is independently verifiable.
CREATE OR REPLACE FUNCTION reject_audit_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % on id=% is forbidden', TG_OP, OLD.id;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_audit_log_immutable ON audit_log;
CREATE TRIGGER trg_audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION reject_audit_mutation();
`

type AuditEntry struct {
	ID        int64           `json:"id"`
	EntryID   string          `json:"entry_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	PrevHash  string          `json:"prev_hash"`
	Hash      string          `json:"hash"`
	CreatedAt time.Time       `json:"created_at"`
}

type TruthCore struct {
	db *pgxpool.Pool
}

func computeHash(prevHash, eventType string, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte{0})
	h.Write([]byte(eventType))
	h.Write([]byte{0})
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func NewTruthCore(dsn string) *TruthCore {
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("truth-core postgres: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := db.Ping(ctx)
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			log.Fatalf("truth-core postgres unavailable after 30s: %v", err)
		}
		log.Println("truth-core: waiting for postgres...")
		time.Sleep(2 * time.Second)
	}

	if _, err := db.Exec(context.Background(), initSQL); err != nil {
		log.Fatalf("truth-core schema: %v", err)
	}
	log.Println("Truth-Core audit log initialized")
	return &TruthCore{db: db}
}

// Append inserts a new audit entry with hash chaining under row-level locking.
func (tc *TruthCore) Append(ctx context.Context, eventType string, payload interface{}) (*AuditEntry, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	tx, err := tc.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Hash the exact canonical representation PostgreSQL stores and later returns.
	var canonicalPayload []byte
	if err := tx.QueryRow(ctx, `SELECT $1::jsonb`, string(payloadBytes)).Scan(&canonicalPayload); err != nil {
		return nil, fmt.Errorf("canonicalize payload: %w", err)
	}
	payloadBytes = canonicalPayload

	// Read the last row's hash under an exclusive lock to prevent races.
	var prevHash string
	row := tx.QueryRow(ctx,
		`SELECT hash FROM audit_log ORDER BY id DESC LIMIT 1 FOR UPDATE`)
	if err := row.Scan(&prevHash); err != nil {
		if err == pgx.ErrNoRows {
			prevHash = "genesis"
		} else {
			return nil, fmt.Errorf("read prev hash: %w", err)
		}
	}

	// Domain-separated hash binds both the event type and canonical JSON payload.
	newHash := computeHash(prevHash, eventType, payloadBytes)

	entryID := uuid.NewString()
	entry := &AuditEntry{
		EntryID:   entryID,
		EventType: eventType,
		Payload:   payloadBytes,
		PrevHash:  prevHash,
		Hash:      newHash,
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO audit_log (entry_id, event_type, payload, prev_hash, hash)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		entryID, eventType, payloadBytes, prevHash, newHash,
	).Scan(&entry.ID, &entry.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return entry, nil
}

// VerifyChain reads the full audit log and verifies every hash link.
// Returns the count of verified entries and any tampering error.
func (tc *TruthCore) VerifyChain(ctx context.Context) (int, error) {
	rows, err := tc.db.Query(ctx,
		`SELECT id, prev_hash, hash, event_type, payload FROM audit_log ORDER BY id ASC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int
	var prevHash = "genesis"

	for rows.Next() {
		var id int64
		var storedPrevHash, storedHash, eventType string
		var payload []byte

		if err := rows.Scan(&id, &storedPrevHash, &storedHash, &eventType, &payload); err != nil {
			return count, err
		}

		if storedPrevHash != prevHash {
			return count, fmt.Errorf("chain broken at id=%d: expected prev=%s got=%s",
				id, prevHash, storedPrevHash)
		}

		expectedHash := computeHash(storedPrevHash, eventType, payload)
		if expectedHash != storedHash {
			return count, fmt.Errorf("hash mismatch at id=%d: stored=%s computed=%s",
				id, storedHash, expectedHash)
		}
		prevHash = storedHash
		count++
	}
	return count, rows.Err()
}

// requireWriteAuth guards /append when TRUTHCORE_WRITE_SECRET is set.
// Without the secret the endpoint stays open (dev mode) but logs a warning
// at startup; deployments set the secret and fail fast via compose.
func requireWriteAuth(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if secret != "" {
			if r.Header.Get("Authorization") != "Bearer "+secret {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (tc *TruthCore) appendHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EventType string          `json:"event_type"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.EventType == "" || len(body.Payload) == 0 {
		http.Error(w, "event_type and payload are required", http.StatusBadRequest)
		return
	}
	entry, err := tc.Append(r.Context(), body.EventType, body.Payload)
	if err != nil {
		log.Printf("truth-core append error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (tc *TruthCore) verifyHandler(w http.ResponseWriter, r *http.Request) {
	count, err := tc.VerifyChain(r.Context())
	resp := map[string]interface{}{"verified_entries": count}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		resp["error"] = err.Error()
		resp["valid"] = false
		w.WriteHeader(http.StatusConflict)
	} else {
		resp["valid"] = true
	}
	json.NewEncoder(w).Encode(resp)
}

func (tc *TruthCore) recentHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := tc.db.Query(r.Context(),
		`SELECT id, entry_id, event_type, payload, prev_hash, hash, created_at
		 FROM audit_log ORDER BY id DESC LIMIT 100`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		rows.Scan(&e.ID, &e.EntryID, &e.EventType, &e.Payload, &e.PrevHash, &e.Hash, &e.CreatedAt)
		entries = append(entries, e)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// headHandler returns the chain tip: the cheap anchor for independent
// continuity checks.
func (tc *TruthCore) headHandler(w http.ResponseWriter, r *http.Request) {
	var h struct {
		ID      int64  `json:"id"`
		EntryID string `json:"entry_id"`
		Hash    string `json:"hash"`
	}
	err := tc.db.QueryRow(r.Context(),
		`SELECT id, entry_id, hash FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&h.ID, &h.EntryID, &h.Hash)
	if err != nil {
		http.Error(w, `{"error":"chain is empty"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h)
}

// entriesHandler returns full entries (including prev_hash and the exact
// canonical payload bytes) for independent verification.
// GET /entries?since_id=<n>&limit=<m> — ids strictly greater than since_id,
// oldest first.
func (tc *TruthCore) entriesHandler(w http.ResponseWriter, r *http.Request) {
	sinceID, _ := strconv.ParseInt(r.URL.Query().Get("since_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := tc.db.Query(r.Context(),
		`SELECT id, entry_id, event_type, payload, prev_hash, hash, created_at
		 FROM audit_log WHERE id > $1 ORDER BY id ASC LIMIT $2`, sinceID, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	entries := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.EntryID, &e.EventType, &e.Payload, &e.PrevHash, &e.Hash, &e.CreatedAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		entries = append(entries, e)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// fetchTradeEntries loads the trade-lifecycle events for reconciliation.
func (tc *TruthCore) fetchTradeEntries(ctx context.Context) ([]AuditEntry, error) {
	rows, err := tc.db.Query(ctx,
		`SELECT id, entry_id, event_type, payload, prev_hash, hash, created_at
		 FROM audit_log WHERE event_type IN ('aegis.approval_issued','vulture.order_submitted','vulture.fill')
		 ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.EntryID, &e.EventType, &e.Payload, &e.PrevHash, &e.Hash, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// reconcileHandler runs fill reconciliation over the trade-lifecycle events.
// 409 + valid=false when any rule is violated; 200 + valid=true when clean.
// An empty trade history reconciles as valid (nothing to check).
func (tc *TruthCore) reconcileHandler(w http.ResponseWriter, r *http.Request) {
	entries, err := tc.fetchTradeEntries(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	report := reconcileEntries(entries)
	w.Header().Set("Content-Type", "application/json")
	if !report.Valid {
		w.WriteHeader(http.StatusConflict)
	}
	json.NewEncoder(w).Encode(report)
}

// lineageHandler returns the immutable lineage for one approval or order:
// approval_issued -> order_submitted -> fill(s), linked by approval_id/order_id.
// GET /lineage?approval_id=<id> or /lineage?order_id=<id>
func (tc *TruthCore) lineageHandler(w http.ResponseWriter, r *http.Request) {
	approvalID := r.URL.Query().Get("approval_id")
	orderID := r.URL.Query().Get("order_id")
	if approvalID == "" && orderID == "" {
		http.Error(w, "approval_id or order_id is required", http.StatusBadRequest)
		return
	}
	entries, err := tc.fetchTradeEntries(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var lineage []AuditEntry
	if orderID != "" {
		// Find the order's approval, then everything linked to both.
		approvalOfOrder := ""
		for _, e := range entries {
			if e.EventType == eventOrderSubmitted && strField(payloadMap(e), "order_id") == orderID {
				approvalOfOrder = strField(payloadMap(e), "approval_id")
				break
			}
		}
		for _, e := range entries {
			p := payloadMap(e)
			switch e.EventType {
			case eventApprovalIssued:
				if strField(p, "approval_id") == approvalOfOrder {
					lineage = append(lineage, e)
				}
			case eventOrderSubmitted, eventFill:
				if strField(p, "order_id") == orderID {
					lineage = append(lineage, e)
				}
			}
		}
	} else {
		for _, e := range entries {
			p := payloadMap(e)
			switch e.EventType {
			case eventApprovalIssued:
				if strField(p, "approval_id") == approvalID {
					lineage = append(lineage, e)
				}
			case eventOrderSubmitted:
				if strField(p, "approval_id") == approvalID {
					lineage = append(lineage, e)
				}
			case eventFill:
				// Fills link via their order; include fills whose order
				// belongs to this approval.
				if oid := strField(p, "order_id"); oid != "" {
					for _, oe := range entries {
						if oe.EventType == eventOrderSubmitted &&
							strField(payloadMap(oe), "order_id") == oid &&
							strField(payloadMap(oe), "approval_id") == approvalID {
							lineage = append(lineage, e)
							break
						}
					}
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lineage)
}

func main() {
	// PAPER/LIVE contract: startup succeeds only for one unambiguous,
	// fully gated runtime mode. Invalid combinations fail closed.
	modelock.RequireMode("truth-core")

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgresql://postgres:omega@postgres:5432/omega"
	}

	tc := NewTruthCore(dsn)

	mux := http.NewServeMux()
	liveHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"live"}`)
	}
	readyHandler := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := tc.db.Ping(ctx); err != nil {
			http.Error(w, `{"status":"not_ready","dependency":"postgres"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ready"}`)
	}
	mux.HandleFunc("/health", liveHandler)
	mux.HandleFunc("/health/live", liveHandler)
	mux.HandleFunc("/health/ready", readyHandler)
	writeSecret := os.Getenv("TRUTHCORE_WRITE_SECRET")
	if writeSecret == "" {
		log.Println("WARNING: TRUTHCORE_WRITE_SECRET is not set — /append accepts unauthenticated writes")
	}
	mux.HandleFunc("/append", requireWriteAuth(writeSecret, tc.appendHandler))
	mux.HandleFunc("/verify", tc.verifyHandler)
	mux.HandleFunc("/recent", tc.recentHandler)
	mux.HandleFunc("/head", tc.headHandler)
	mux.HandleFunc("/entries", tc.entriesHandler)
	mux.HandleFunc("/reconcile", tc.reconcileHandler)
	mux.HandleFunc("/lineage", tc.lineageHandler)

	go func() {
		log.Println("Truth-Core HTTP on :8084")
		http.ListenAndServe(":8084", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
