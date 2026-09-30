package main

// audit-halt-clear — the operator-only recovery path for a durable audit
// halt (see audit_halt.go). It runs as a subcommand of the execution
// binary from an operator shell:
//
//	execution-service audit-halt-clear --operator NAME [--acknowledge ORDER_ID ...]
//
// It is deliberately NOT an HTTP endpoint, NOT reachable from the web UI,
// and NOT on any order path — the same operator-only posture as the
// risk-service control plane, one shell further in. Recovery semantics:
//
//  1. Every durable gap record is backfilled first: its exact stored
//     vulture.fill payload is appended to TruthCore (marked backfilled).
//  2. Any gap that still cannot be appended must be explicitly
//     acknowledged, order by order (--acknowledge). Each acknowledged gap
//     gets an explicit vulture.fill_audit_gap record in the chain — the
//     record reconciliation accepts in place of the missing fill.
//     (Orders with no durable gap record at all — e.g. flagged by
//     reconciliation after a crash mid-engage — can also be acknowledged
//     this way; the gap record says so.)
//  3. Only then is vulture.audit_halt_cleared appended (operator, what was
//     backfilled, what was acknowledged). If that append fails, NOTHING is
//     cleared: fail closed, exit 1.
//  4. Only after the audit events are durable is the Redis halt state
//     deleted. The running execution-service observes the clear on its
//     next submission (it re-reads the durable state per signal).
//
// Exit codes: 0 = cleared (or nothing to clear), 1 = NOT cleared,
// 2 = usage error.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/omega-prime-delta/modelock"
	"github.com/omega-prime-delta/truthclient"
)

type repeatFlag []string

func (f *repeatFlag) String() string { return fmt.Sprint([]string(*f)) }
func (f *repeatFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

type clearSummary struct {
	Halted       bool
	Backfilled   []string
	Acknowledged []string
}

func runAuditHaltClear(args []string) int {
	fs := flag.NewFlagSet("audit-halt-clear", flag.ContinueOnError)
	operator := fs.String("operator", os.Getenv("OPERATOR"),
		"operator identity recorded in the audit events (required: the clear is itself audited)")
	var ackIDs repeatFlag
	fs.Var(&ackIDs, "acknowledge",
		"order_id of an unbackfilled fill gap the operator explicitly acknowledges (repeatable)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: execution-service audit-halt-clear --operator NAME [--acknowledge ORDER_ID ...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *operator == "" {
		fmt.Fprintln(os.Stderr, "audit-halt-clear: --operator is required (WHO cleared the halt is part of the audit record)")
		return 2
	}

	// Same PAPER/LIVE lock as the service: this binary never runs outside
	// paper mode, operator command included.
	modelock.RequirePaper("execution-service audit-halt-clear")

	truthURL := os.Getenv("TRUTHCORE_URL")
	if truthURL == "" {
		truthURL = "http://truth-core:8084"
	}
	rdb := newRedisClient(os.Getenv("REDIS_URL"))
	defer rdb.Close()
	tc := truthclient.New(truthURL, os.Getenv("TRUTHCORE_WRITE_SECRET"))

	sum, err := clearAuditHalt(context.Background(), rdb, tc, *operator, ackIDs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit-halt-clear: NOT CLEARED (fail closed): %v\n", err)
		return 1
	}
	if !sum.Halted && len(sum.Backfilled) == 0 && len(sum.Acknowledged) == 0 {
		fmt.Println("audit-halt-clear: no audit halt in effect, no gap records — nothing to do")
		return 0
	}
	fmt.Printf("audit-halt-clear: cleared by %s — backfilled: %v, acknowledged: %v\n",
		*operator, sum.Backfilled, sum.Acknowledged)
	return 0
}

// clearAuditHalt performs the recovery. It never deletes durable state
// before the corresponding audit events are appended, and it reports
// partial progress (successful backfills stay appended — they are correct
// chain entries) alongside any error.
func clearAuditHalt(ctx context.Context, rdb *redis.Client, tc *truthclient.Client, operator string, acknowledge []string) (*clearSummary, error) {
	haltVal, err := rdb.Get(ctx, auditHaltedKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("read halt state: %w", err)
	}
	halted := haltVal == "1"
	gaps, err := redisGapRecords(ctx, rdb)
	if err != nil {
		return nil, err
	}

	sum := &clearSummary{Halted: halted, Backfilled: []string{}, Acknowledged: []string{}}
	if !halted && len(gaps) == 0 && len(acknowledge) == 0 {
		return sum, nil
	}

	appendEvent := func(eventType string, payload map[string]interface{}) error {
		actx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := tc.Append(actx, eventType, payload)
		return err
	}

	// Step 1: backfill every gap whose fill payload we still hold.
	remaining := map[string]fillGapRecord{}
	for orderID, rec := range gaps {
		payload := make(map[string]interface{}, len(rec.Fill)+2)
		for k, v := range rec.Fill {
			payload[k] = v
		}
		payload["backfilled"] = true
		payload["backfilled_at_ms"] = time.Now().UnixMilli()
		if err := appendEvent(truthclient.EventFill, payload); err != nil {
			remaining[orderID] = rec
			continue
		}
		if err := rdb.HDel(ctx, auditGapsKey, orderID).Err(); err != nil {
			return sum, fmt.Errorf("gap %s backfilled but its gap record could not be deleted: %w", orderID, err)
		}
		sum.Backfilled = append(sum.Backfilled, orderID)
	}

	// Step 2: every remaining gap must be explicitly acknowledged,
	// order by order. No acknowledgement, no clear.
	ackWanted := map[string]bool{}
	for _, id := range acknowledge {
		ackWanted[id] = true
	}
	var unacked []string
	for orderID := range remaining {
		if !ackWanted[orderID] {
			unacked = append(unacked, orderID)
		}
	}
	if len(unacked) > 0 {
		sort.Strings(unacked)
		return sum, fmt.Errorf("unrecovered fill gaps not acknowledged: %v — restore TruthCore and retry, or re-run with --acknowledge <order_id> for each", unacked)
	}
	backfilled := map[string]bool{}
	for _, id := range sum.Backfilled {
		backfilled[id] = true
	}
	ackList := []string{}
	for _, id := range acknowledge {
		if backfilled[id] {
			continue // already backfilled; no gap record needed
		}
		dup := false
		for _, done := range ackList {
			if done == id {
				dup = true
			}
		}
		if dup {
			continue
		}
		rec, hasRecord := remaining[id]
		payload := map[string]interface{}{
			"order_id":             id,
			"acknowledged_by":      operator,
			"acknowledged_at_ms":   time.Now().UnixMilli(),
			"gap_record_persisted": hasRecord,
		}
		if hasRecord {
			payload["approval_id"] = rec.Fill["approval_id"]
			payload["signal_id"] = rec.Fill["signal_id"]
			payload["original_error"] = rec.LastError
			payload["fill_attempts"] = rec.Attempts
		}
		if err := appendEvent(truthclient.EventFillAuditGap, payload); err != nil {
			return sum, fmt.Errorf("append fill_audit_gap for order %s: %w", id, err)
		}
		ackList = append(ackList, id)
	}
	sort.Strings(ackList)
	sum.Acknowledged = ackList
	sort.Strings(sum.Backfilled)

	// Step 3: the clear itself is audited BEFORE any durable state is
	// cleared. If this append fails, the halt stays — fail closed.
	if halted || len(gaps) > 0 {
		payload := map[string]interface{}{
			"operator":               operator,
			"cleared_at_ms":          time.Now().UnixMilli(),
			"backfilled_order_ids":   sum.Backfilled,
			"acknowledged_order_ids": ackList,
			"unbackfilled_gaps":      len(remaining),
		}
		if err := appendEvent(truthclient.EventAuditHaltCleared, payload); err != nil {
			return sum, fmt.Errorf("append audit_halt_cleared (halt NOT cleared): %w", err)
		}

		// Step 4: only now delete the durable halt state (halt flag +
		// acknowledged gap records; backfilled ones were HDELed above).
		if err := rdb.Del(ctx, auditGapsKey, auditHaltedKey).Err(); err != nil {
			return sum, fmt.Errorf("clear event is audited but durable state delete failed: %w", err)
		}
	}
	return sum, nil
}
