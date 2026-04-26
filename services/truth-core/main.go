package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/segmentio/kafka-go"
)

type AuditEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

func chainHash(prev string, evt []byte) string {
	h := sha256.Sum256(append([]byte(prev), evt...))
	return hex.EncodeToString(h[:])
}

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "kafka:9092"
	}
	r := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{brokers}, Topic: "audit.events", GroupID: "truth-core"})
	defer r.Close()

	prevHash := "GENESIS"
	for {
		msg, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}
		var evt AuditEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			log.Printf("invalid event: %v", err)
			continue
		}
		hash := chainHash(prevHash, msg.Value)
		_, err = db.Exec(`INSERT INTO audit_log (event_id, event_type, payload, prev_hash, hash, created_at)
			VALUES ($1,$2,$3,$4,$5,NOW())`, evt.ID, evt.Type, string(msg.Value), prevHash, hash)
		if err != nil {
			log.Printf("insert failed: %v", err)
			continue
		}
		prevHash = hash
	}
}
