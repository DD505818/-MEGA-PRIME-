package main

// Phase 1B.2 — position reconciliation.
//
// Execution-service maintains three pieces of position state in Redis:
//   portfolio:position:<symbol>  net quantity (incremental IncrByFloat)
//   portfolio:fills:<symbol>     immutable append-only fills ledger
//   portfolio:open_symbols       set of symbols with nonzero net position
//   portfolio:tracked_symbols    set of symbols ever filled
//
// Every 60s this reconciler recomputes each symbol's expected quantity from
// the fills ledger and diffs it against the live position key, and checks
// that open_symbols agrees with the position keys. Any divergence means the
// position state cannot be trusted — Gate 8 (max open positions) would be
// deciding on corrupt input — so the reconciler fails closed: audit the
// divergence, page via risk.alerts, and activate the kill switch.
//
// For paper mode the fills ledger is the independent "broker snapshot": it
// is written before the position key on every fill, so a crash or partial
// write between the two surfaces here as a divergence instead of silent
// drift. (Live mode has no broker snapshot yet — live is locked.)

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// positionReconcileInterval matches the Phase 1A reconciliation cadence.
const positionReconcileInterval = 60 * time.Second

// positionQtyEpsilonRecon treats dust-level float differences as agreement.
const positionQtyEpsilonRecon = 1e-9

func (r *RiskEngine) reconcilePositionsLoop() {
	t := time.NewTicker(positionReconcileInterval)
	defer t.Stop()
	for range t.C {
		r.reconcilePositions()
	}
}

// reconcilePositions runs one reconcile pass. It returns the divergences
// found (empty = healthy). Any divergence fails closed: the kill switch is
// activated, because Gate 8 cannot be trusted on divergent state.
func (r *RiskEngine) reconcilePositions() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	symbols := map[string]struct{}{}
	for _, s := range r.redis.SMembers(ctx, "portfolio:tracked_symbols").Val() {
		symbols[s] = struct{}{}
	}
	openSyms := map[string]bool{}
	for _, s := range r.redis.SMembers(ctx, "portfolio:open_symbols").Val() {
		symbols[s] = struct{}{}
		openSyms[s] = true
	}

	var divergences []string
	for sym := range symbols {
		expected := 0.0
		for _, raw := range r.redis.LRange(ctx, "portfolio:fills:"+sym, 0, -1).Val() {
			var rec struct {
				Side string  `json:"side"`
				Qty  float64 `json:"qty"`
			}
			if err := json.Unmarshal([]byte(raw), &rec); err != nil {
				divergences = append(divergences, fmt.Sprintf("%s: unparseable ledger entry", sym))
				continue
			}
			if rec.Side == "BUY" {
				expected += rec.Qty
			} else {
				expected -= rec.Qty
			}
		}
		actual := r.redisFloat(ctx, "portfolio:position:"+sym)
		if math.Abs(expected-actual) > positionQtyEpsilonRecon {
			divergences = append(divergences,
				fmt.Sprintf("%s: ledger=%.9f position=%.9f", sym, expected, actual))
			continue
		}
		if openSyms[sym] != (math.Abs(actual) > positionQtyEpsilonRecon) {
			divergences = append(divergences,
				fmt.Sprintf("%s: open_symbols=%t but position=%.9f", sym, openSyms[sym], actual))
		}
	}

	if len(divergences) > 0 {
		detail := strings.Join(divergences, "; ")
		log.Printf("POSITION DIVERGENCE: %s — failing closed", detail)
		r.recordControlAudit("risk.gate8", "system:position-reconciler", true, "divergence", detail)
		r.publishAlert(ctx, "POSITION_DIVERGENCE", map[string]interface{}{"divergences": divergences})
		if _, err := r.activateKillSwitch("1B2_POSITION_DIVERGENCE"); err != nil {
			log.Printf("POSITION DIVERGENCE kill failed: %v", err)
		}
	}
	return divergences
}
