package main

// Durable audit events for control-plane actions (kill / reset).
//
// Every attempt — granted or denied — is recorded in three places, in order
// of reliability:
//
//  1. Structured stdout log line (always; survives everything else).
//  2. Redis stream "control:audit" (durable while Redis persists).
//  3. TruthCore hash-chained audit log via POST /append (best-effort).
//
// TruthCore is never allowed to block a control action, so its appends can
// fail. A failed append is NOT silently dropped: the hole in the hash chain
// is itself recorded in the Redis stream "control:audit:gaps" with the
// audit_id, the failure time, and the reason, so a reconciler can backfill
// the chain and a reviewer can see exactly which events are missing. If
// Redis is also down, the stdout log line (layer 1) still carries the
// audit_id for manual reconciliation.
//
// Recording an audit event must never block or fail the control action it
// describes: all remote writes are best-effort with short timeouts and
// nil-safe receivers.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	controlAuditStream    = "control:audit"
	controlAuditGapStream = "control:audit:gaps"
)

// recordControlAudit records a control-plane attempt. Safe to call with a nil
// engine (tests) — the structured log line is always emitted.
func (r *RiskEngine) recordControlAudit(action, actor string, authorized bool, outcome, reason string) {
	auditID := uuid.NewString()
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	log.Printf("CONTROL_AUDIT audit_id=%s ts=%s action=%s actor=%s authorized=%t outcome=%s reason=%q",
		auditID, ts, action, actor, authorized, outcome, reason)

	if r == nil || r.redis == nil {
		return
	}

	fields := map[string]interface{}{
		"audit_id":   auditID,
		"ts":         ts,
		"action":     action,
		"actor":      actor,
		"authorized": fmt.Sprintf("%t", authorized),
		"outcome":    outcome,
		"reason":     reason,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: controlAuditStream,
		Values: fields,
	}).Err(); err != nil {
		log.Printf("CONTROL_AUDIT stream write failed (non-fatal): %v", err)
	}

	if err := r.postControlAuditToTruthCore(action, fields); err != nil {
		r.recordAuditGap(auditID, action, actor, outcome, err)
	}
}

// recordAuditGap records a TruthCore append failure so the hole in the hash
// chain is visible to reconcilers and reviewers instead of silent.
func (r *RiskEngine) recordAuditGap(auditID, action, actor, outcome string, tcErr error) {
	failedAt := time.Now().UTC().Format(time.RFC3339Nano)
	log.Printf("CONTROL_AUDIT_GAP audit_id=%s action=%s failed_at=%s reason=%q",
		auditID, action, failedAt, tcErr.Error())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	gap := map[string]interface{}{
		"audit_id":  auditID,
		"ts":        failedAt,
		"failed_at": failedAt,
		"action":    action,
		"actor":     actor,
		"outcome":   outcome,
		"reason":    tcErr.Error(),
	}
	if err := r.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: controlAuditGapStream,
		Values: gap,
	}).Err(); err != nil {
		log.Printf("CONTROL_AUDIT gap stream write failed (non-fatal): %v", err)
	}
}

// postControlAuditToTruthCore appends the event to the TruthCore hash chain.
// Best-effort and non-blocking; returns the error so the caller can record
// the gap. (Authentication of this internal append is a known gap — the
// TruthCore /append endpoint currently accepts unauthenticated writes from
// inside the cluster; see the Phase 1A review notes.)
func (r *RiskEngine) postControlAuditToTruthCore(action string, fields map[string]interface{}) error {
	base := os.Getenv("TRUTHCORE_URL")
	if base == "" {
		base = "http://truth-core:8084"
	}
	body, err := json.Marshal(map[string]interface{}{
		"event_type": "control." + action,
		"payload":    fields,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/append", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("truth-core append: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("truth-core append status %d", resp.StatusCode)
	}
	return nil
}
