package main

// Durable audit halt (TruthCore durability fix #7).
//
// Audit-or-no-trade has a hole the moment a fill happens: the fill cannot
// be un-happened, so if its TruthCore append fails, the trade exists
// without its audit record. The original mitigation — an in-memory flag
// that halts new submissions — died with the process: a restart silently
// resumed trading over an unrecorded fill that reconciliation could not
// even see (it had no submitted-order-without-fill rule).
//
// This file makes the halt durable in Redis (the same store the kill
// switch already trusts):
//
//   - A failed fill append gets a bounded retry (appendFillWithRetry).
//   - If it still fails, the order's full fill payload is written as a
//     durable gap record, and the halt flag is set — in Redis FIRST, so
//     every process (and every restart) refuses new submissions with
//     AUDIT_HALTED until an operator clears the halt.
//   - Halt state is authoritative as: halt flag set OR >=1 gap record
//     exists. Both are written at engage time; both are deleted only by
//     the operator clear path (audit_clear.go), which appends the
//     backfilled fills / acknowledged gap records and its own
//     audit_halt_cleared event to TruthCore BEFORE touching Redis.
//
// Residual (documented, detection-covered): if Redis itself is unreachable
// at the exact moment of the gap, the durable writes cannot land; the
// in-memory flag still halts this process, CRITICAL logs fire, and after
// a restart TruthCore reconciliation flags the order ORDER_WITHOUT_FILL.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// Durable audit-halt state in Redis. execution: namespace, matching the
// service's existing portfolio:* keys.
const (
	// auditHaltedKey is "1" while a durable audit halt is in effect.
	auditHaltedKey = "execution:audit_halted"
	// auditGapsKey is a hash: field = order_id, value = JSON fillGapRecord.
	auditGapsKey = "execution:audit_gaps"
)

// fillAuditAttempts bounds the inline retry of a fill's TruthCore append
// before a durable gap is recorded. The retry rides out brief TruthCore
// restarts/blips without operator involvement.
const fillAuditAttempts = 3

// fillAuditRetryDelay is the base backoff between fill-audit attempts
// (delay * attempt). A var so tests can shrink it.
var fillAuditRetryDelay = 200 * time.Millisecond

// fillGapRecord is the durable record of a fill whose TruthCore append
// failed permanently (after bounded retry). It carries everything needed
// to backfill the exact vulture.fill event later.
type fillGapRecord struct {
	OrderID      string                 `json:"order_id"`
	Fill         map[string]interface{} `json:"fill"` // exact vulture.fill payload
	Attempts     int                    `json:"attempts"`
	LastError    string                 `json:"last_error"`
	RecordedAtMs int64                  `json:"recorded_at_ms"`
}

// fillPayload builds the canonical vulture.fill payload for an order.
// recordFill and the gap record both use it, so a backfilled fill is
// byte-identical in shape to a live-appended one.
func (e *ExecutionEngine) fillPayload(order *Order) map[string]interface{} {
	approvalID, _ := order.Meta["approval_id"].(string)
	return map[string]interface{}{
		"order_id":     order.ID,
		"signal_id":    order.SignalID,
		"strategy_id":  order.StrategyID,
		"approval_id":  approvalID,
		"symbol":       order.Symbol,
		"side":         order.Side,
		"quantity":     order.FilledQty,
		"fill_price":   order.AvgFill,
		"filled_at_ms": order.UpdatedAt,
	}
}

// appendFillWithRetry appends the fill to TruthCore with bounded retry.
// A nil error means the fill is durably audited.
func (e *ExecutionEngine) appendFillWithRetry(order *Order) error {
	var err error
	for attempt := 1; attempt <= fillAuditAttempts; attempt++ {
		if err = e.recordFill(order); err == nil {
			if attempt > 1 {
				log.Printf("ORDER %s: fill audit recovered on attempt %d/%d", order.ID[:8], attempt, fillAuditAttempts)
			}
			return nil
		}
		log.Printf("ORDER %s: fill audit attempt %d/%d failed: %v", order.ID[:8], attempt, fillAuditAttempts, err)
		if attempt < fillAuditAttempts {
			time.Sleep(fillAuditRetryDelay * time.Duration(attempt))
		}
	}
	return err
}

// engageAuditHalt records the fill gap durably and engages the halt.
// Order of writes is deliberate: the gap record and the halt flag both
// make the halt authoritative, so whichever lands first fails closed.
// Write failures are CRITICAL-logged; the in-memory flag still halts this
// process, and reconciliation detects any residue after a restart.
func (e *ExecutionEngine) engageAuditHalt(order *Order, cause error) {
	rec := fillGapRecord{
		OrderID:      order.ID,
		Fill:         e.fillPayload(order),
		Attempts:     fillAuditAttempts,
		LastError:    cause.Error(),
		RecordedAtMs: time.Now().UnixMilli(),
	}
	raw, merr := json.Marshal(rec)
	if merr != nil {
		log.Printf("CRITICAL FILL_AUDIT_GAP order_id=%s: gap record marshal failed: %v", order.ID, merr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gapPersisted, haltPersisted := false, false
	for attempt := 1; attempt <= 3; attempt++ {
		if !gapPersisted && merr == nil {
			gapPersisted = e.redis.HSet(ctx, auditGapsKey, order.ID, raw).Err() == nil
		}
		if !haltPersisted {
			haltPersisted = e.redis.Set(ctx, auditHaltedKey, "1", 0).Err() == nil
		}
		if gapPersisted && haltPersisted {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !gapPersisted || !haltPersisted {
		// Redis itself is failing: the durable record could not land. This
		// process still halts (in-memory flag below); the gap is visible in
		// CRITICAL logs now and via ORDER_WITHOUT_FILL reconciliation after
		// any restart.
		log.Printf("CRITICAL FILL_AUDIT_GAP order_id=%s: durable halt state NOT fully persisted (gap=%v halt=%v) — Redis failing; in-memory halt only", order.ID, gapPersisted, haltPersisted)
	}
	e.auditHalted.Store(true)
}

// auditHaltActive reports whether new submissions are halted by the
// durable audit-halt state. The running process observes an operator
// clear (which deletes the Redis state) on the next submission. A Redis
// read failure is fail-closed: halted if the in-memory flag says so,
// otherwise an error so the caller refuses with AUDIT_STATE_UNKNOWN.
func (e *ExecutionEngine) auditHaltActive(ctx context.Context) (bool, error) {
	v, err := e.redis.Get(ctx, auditHaltedKey).Result()
	switch {
	case err == nil && v == "1":
		e.auditHalted.Store(true)
		return true, nil
	case err == nil || err == redis.Nil:
		// No halt flag: gap records alone are also authoritative (they are
		// only ever created by engageAuditHalt and only removed after the
		// fill is backfilled or explicitly acknowledged).
		n, herr := e.redis.HLen(ctx, auditGapsKey).Result()
		if herr != nil {
			if e.auditHalted.Load() {
				return true, nil
			}
			return false, herr
		}
		halted := n > 0
		e.auditHalted.Store(halted)
		return halted, nil
	default:
		if e.auditHalted.Load() {
			return true, nil
		}
		return false, err
	}
}

// restoreAuditHaltState re-arms the in-memory halt flag from the durable
// Redis state at boot, mirroring risk-service's restoreControlState: if
// the durable state cannot be read, assume HALTED (fail closed) until it
// can. The submit path re-reads Redis per signal regardless; this makes
// the boot state explicit and logged.
func (e *ExecutionEngine) restoreAuditHaltState() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	halted, err := e.auditHaltActive(ctx)
	if err != nil {
		e.auditHalted.Store(true)
		log.Printf("audit-halt: durable halt state unreadable at boot (%v) — assuming HALTED (fail-closed)", err)
		return
	}
	if halted {
		n, _ := e.redis.HLen(ctx, auditGapsKey).Result()
		log.Printf("audit-halt: durable audit halt restored at boot (%d unrecovered fill gap(s)) — submissions refused (AUDIT_HALTED) until an operator runs: execution-service audit-halt-clear", n)
	}
}

// redisGapRecords reads all durable gap records (operator path helper).
func redisGapRecords(ctx context.Context, rdb *redis.Client) (map[string]fillGapRecord, error) {
	raw, err := rdb.HGetAll(ctx, auditGapsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("read gap records: %w", err)
	}
	gaps := make(map[string]fillGapRecord, len(raw))
	for orderID, s := range raw {
		var rec fillGapRecord
		if err := json.Unmarshal([]byte(s), &rec); err != nil {
			return nil, fmt.Errorf("gap record for order %s is unparsable — refusing to proceed over it: %w", orderID, err)
		}
		gaps[orderID] = rec
	}
	return gaps, nil
}
