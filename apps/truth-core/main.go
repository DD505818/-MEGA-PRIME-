// Truth-Core — append-only, SHA-256 hash-chained audit log.
//
// Every INSERT reads the previous row's hash under SELECT ... FOR UPDATE,
// computes SHA-256(prev_hash || record_json), and commits atomically.
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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func NewTruthCore(dsn string) *TruthCore {
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("truth-core postgres: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		if err := db.Ping(ctx); err == nil {
			break
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

	// Compute SHA-256(prev_hash || payload)
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(payloadBytes)
	newHash := hex.EncodeToString(h.Sum(nil))

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
		`SELECT id, prev_hash, hash, payload FROM audit_log ORDER BY id ASC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int
	var prevHash = "genesis"

	for rows.Next() {
		var id int64
		var storedPrevHash, storedHash string
		var payload []byte

		if err := rows.Scan(&id, &storedPrevHash, &storedHash, &payload); err != nil {
			return count, err
		}

		if storedPrevHash != prevHash {
			return count, fmt.Errorf("chain broken at id=%d: expected prev=%s got=%s",
				id, prevHash, storedPrevHash)
		}

		h := sha256.New()
		h.Write([]byte(storedPrevHash))
		h.Write(payload)
		expectedHash := hex.EncodeToString(h.Sum(nil))
		if expectedHash != storedHash {
			return count, fmt.Errorf("hash mismatch at id=%d: stored=%s computed=%s",
				id, storedHash, expectedHash)
		}
		prevHash = storedHash
		count++
	}
	return count, rows.Err()
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func (tc *TruthCore) appendHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EventType string          `json:"event_type"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", 400)
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
	if err != nil {
		resp["error"] = err.Error()
		resp["valid"] = false
		w.WriteHeader(409)
	} else {
		resp["valid"] = true
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (tc *TruthCore) recentHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := tc.db.Query(r.Context(),
		`SELECT id, entry_id, event_type, payload, hash, created_at
		 FROM audit_log ORDER BY id DESC LIMIT 100`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		rows.Scan(&e.ID, &e.EntryID, &e.EventType, &e.Payload, &e.Hash, &e.CreatedAt)
		entries = append(entries, e)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgresql://postgres:omega@postgres:5432/omega"
	}

	tc := NewTruthCore(dsn)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/append", tc.appendHandler)
	mux.HandleFunc("/verify", tc.verifyHandler)
	mux.HandleFunc("/recent", tc.recentHandler)

	go func() {
		log.Println("Truth-Core HTTP on :8084")
		http.ListenAndServe(":8084", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
