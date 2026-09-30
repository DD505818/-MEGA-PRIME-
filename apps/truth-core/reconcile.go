package main

// Fill reconciliation: independent cross-checks over the trade-lifecycle
// events in the audit log.
//
// Rules:
//  1. Every vulture.order_submitted must reference exactly one
//     aegis.approval_issued (by approval_id), with matching bound fields.
//  2. Every aegis.approval_issued must back at most one order_submitted
//     (double-spend detection — approvals are single-use).
//  3. Every vulture.fill must reference exactly one order_submitted
//     (by order_id), with matching approval_id and quantity.
//
// reconcileEntries is pure: it takes decoded entries and returns a report,
// so the rules are unit-testable without a database.

import (
	"encoding/json"
	"fmt"
	"time"
)

// Canonical trade-lifecycle event types (must match truthclient constants).
const (
	eventApprovalIssued = "aegis.approval_issued"
	eventOrderSubmitted = "vulture.order_submitted"
	eventFill           = "vulture.fill"
)

type Violation struct {
	Rule    string  `json:"rule"`
	Detail  string  `json:"detail"`
	EntryIDs []int64 `json:"entry_ids"`
}

type ReconcileReport struct {
	CheckedAt       time.Time   `json:"checked_at"`
	ApprovalsIssued int         `json:"approvals_issued"`
	OrdersSubmitted int         `json:"orders_submitted"`
	Fills           int         `json:"fills"`
	Violations      []Violation `json:"violations"`
	Valid           bool        `json:"valid"`
}

func payloadMap(e AuditEntry) map[string]interface{} {
	var m map[string]interface{}
	_ = json.Unmarshal(e.Payload, &m)
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

func strField(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

func f64Field(m map[string]interface{}, key string) (float64, bool) {
	f, ok := m[key].(float64)
	return f, ok
}

func reconcileEntries(entries []AuditEntry) ReconcileReport {
	report := ReconcileReport{
		CheckedAt:  time.Now().UTC(),
		Violations: []Violation{},
	}

	approvals := map[string]AuditEntry{}   // approval_id -> entry
	approvalPayloads := map[string]map[string]interface{}{}
	orders := map[string]AuditEntry{}      // order_id -> entry
	orderPayloads := map[string]map[string]interface{}{}
	ordersByApproval := map[string][]string{} // approval_id -> order_ids

	for _, e := range entries {
		p := payloadMap(e)
		switch e.EventType {
		case eventApprovalIssued:
			if id := strField(p, "approval_id"); id != "" {
				approvals[id] = e
				approvalPayloads[id] = p
			}
		case eventOrderSubmitted:
			oid := strField(p, "order_id")
			aid := strField(p, "approval_id")
			if oid == "" {
				continue
			}
			orders[oid] = e
			orderPayloads[oid] = p
			if aid != "" {
				ordersByApproval[aid] = append(ordersByApproval[aid], oid)
			}
		}
	}
	report.ApprovalsIssued = len(approvals)
	report.OrdersSubmitted = len(orders)

	// Rule 1: every order must reference exactly one issued approval, with
	// matching bound fields.
	for oid, e := range orders {
		p := orderPayloads[oid]
		aid := strField(p, "approval_id")
		ae, ok := approvals[aid]
		if !ok {
			report.Violations = append(report.Violations, Violation{
				Rule:     "ORDER_WITHOUT_APPROVAL",
				Detail:   fmt.Sprintf("order %s references unknown approval %q", oid, aid),
				EntryIDs: []int64{e.ID},
			})
			continue
		}
		ap := approvalPayloads[aid]
		mismatches := []string{}
		for _, field := range []string{"symbol", "side", "mode"} {
			if strField(p, field) != strField(ap, field) {
				mismatches = append(mismatches, field)
			}
		}
		if q1, ok1 := f64Field(p, "quantity"); ok1 {
			if q2, ok2 := f64Field(ap, "quantity"); !ok2 || q1 != q2 {
				mismatches = append(mismatches, "quantity")
			}
		}
		if len(mismatches) > 0 {
			report.Violations = append(report.Violations, Violation{
				Rule:     "ORDER_APPROVAL_MISMATCH",
				Detail:   fmt.Sprintf("order %s diverges from approval %s on: %v", oid, aid, mismatches),
				EntryIDs: []int64{e.ID, ae.ID},
			})
		}
	}

	// Rule 2: single-use — one approval backs at most one order.
	for aid, oids := range ordersByApproval {
		if len(oids) > 1 {
			ids := []int64{}
			if ae, ok := approvals[aid]; ok {
				ids = append(ids, ae.ID)
			}
			for _, oid := range oids {
				ids = append(ids, orders[oid].ID)
			}
			report.Violations = append(report.Violations, Violation{
				Rule:     "APPROVAL_DOUBLE_SPEND",
				Detail:   fmt.Sprintf("approval %s backs %d orders: %v", aid, len(oids), oids),
				EntryIDs: ids,
			})
		}
	}

	// Rule 3: every fill must match one submitted order.
	for _, e := range entries {
		if e.EventType != eventFill {
			continue
		}
		report.Fills++
		p := payloadMap(e)
		oid := strField(p, "order_id")
		oe, ok := orders[oid]
		if !ok {
			report.Violations = append(report.Violations, Violation{
				Rule:     "FILL_WITHOUT_ORDER",
				Detail:   fmt.Sprintf("fill references unknown order %q", oid),
				EntryIDs: []int64{e.ID},
			})
			continue
		}
		op := orderPayloads[oid]
		if strField(p, "approval_id") != strField(op, "approval_id") {
			report.Violations = append(report.Violations, Violation{
				Rule:     "FILL_APPROVAL_MISMATCH",
				Detail:   fmt.Sprintf("fill for order %s carries different approval_id than the order", oid),
				EntryIDs: []int64{e.ID, oe.ID},
			})
		}
		if q1, ok1 := f64Field(p, "quantity"); ok1 {
			if q2, ok2 := f64Field(op, "quantity"); !ok2 || q1 != q2 {
				report.Violations = append(report.Violations, Violation{
					Rule:     "FILL_QTY_MISMATCH",
					Detail:   fmt.Sprintf("fill quantity %v != order quantity %v for order %s", q1, q2, oid),
					EntryIDs: []int64{e.ID, oe.ID},
				})
			}
		}
	}

	report.Valid = len(report.Violations) == 0
	return report
}
