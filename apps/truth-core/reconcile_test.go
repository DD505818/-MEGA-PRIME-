package main

import (
	"encoding/json"
	"testing"
)

func mkEntry(id int64, eventType string, payload map[string]interface{}) AuditEntry {
	b, _ := json.Marshal(payload)
	return AuditEntry{ID: id, EntryID: "e", EventType: eventType, Payload: b}
}

func approvalPayload() map[string]interface{} {
	return map[string]interface{}{
		"approval_id": "a1", "signal_id": "s1", "strategy_id": "st1",
		"symbol": "BTC/USD", "side": "buy", "quantity": 0.01,
		"limit_price": 90000.0, "stop_price": 89000.0, "mode": "paper",
	}
}

func orderPayload() map[string]interface{} {
	return map[string]interface{}{
		"order_id": "o1", "approval_id": "a1",
		"symbol": "BTC/USD", "side": "buy", "quantity": 0.01,
		"limit_price": 90000.0, "stop_price": 89000.0, "mode": "paper",
	}
}

func fillPayload() map[string]interface{} {
	return map[string]interface{}{
		"order_id": "o1", "approval_id": "a1",
		"symbol": "BTC/USD", "side": "buy", "quantity": 0.01,
		"fill_price": 90001.0,
	}
}

func rulesOf(r ReconcileReport) map[string]int {
	m := map[string]int{}
	for _, v := range r.Violations {
		m[v.Rule]++
	}
	return m
}

func TestReconcile_CleanLifecycle(t *testing.T) {
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventOrderSubmitted, orderPayload()),
		mkEntry(3, eventFill, fillPayload()),
	}
	r := reconcileEntries(entries)
	if !r.Valid {
		t.Fatalf("clean lifecycle reported violations: %+v", r.Violations)
	}
	if r.ApprovalsIssued != 1 || r.OrdersSubmitted != 1 || r.Fills != 1 {
		t.Fatalf("counts wrong: %+v", r)
	}
}

func TestReconcile_FillWithoutOrder(t *testing.T) {
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventFill, fillPayload()), // no order_submitted
	}
	r := reconcileEntries(entries)
	if r.Valid {
		t.Fatal("fill without order not detected")
	}
	if rulesOf(r)["FILL_WITHOUT_ORDER"] != 1 {
		t.Fatalf("wrong rules: %+v", r.Violations)
	}
}

func TestReconcile_OrderWithoutApproval(t *testing.T) {
	op := orderPayload()
	op["approval_id"] = "unknown-approval"
	entries := []AuditEntry{
		mkEntry(1, eventOrderSubmitted, op),
	}
	r := reconcileEntries(entries)
	if r.Valid {
		t.Fatal("order without approval not detected")
	}
	if rulesOf(r)["ORDER_WITHOUT_APPROVAL"] != 1 {
		t.Fatalf("wrong rules: %+v", r.Violations)
	}
}

func TestReconcile_ApprovalDoubleSpend(t *testing.T) {
	op2 := orderPayload()
	op2["order_id"] = "o2"
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventOrderSubmitted, orderPayload()),
		mkEntry(3, eventOrderSubmitted, op2), // same approval, second order
	}
	r := reconcileEntries(entries)
	if r.Valid {
		t.Fatal("approval double-spend not detected")
	}
	if rulesOf(r)["APPROVAL_DOUBLE_SPEND"] != 1 {
		t.Fatalf("wrong rules: %+v", r.Violations)
	}
}

func TestReconcile_OrderApprovalMismatch(t *testing.T) {
	op := orderPayload()
	op["quantity"] = 0.05 // tampered quantity vs approval
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventOrderSubmitted, op),
	}
	r := reconcileEntries(entries)
	if r.Valid {
		t.Fatal("order/approval field mismatch not detected")
	}
	if rulesOf(r)["ORDER_APPROVAL_MISMATCH"] != 1 {
		t.Fatalf("wrong rules: %+v", r.Violations)
	}
}

func TestReconcile_FillQtyMismatch(t *testing.T) {
	fp := fillPayload()
	fp["quantity"] = 0.02
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventOrderSubmitted, orderPayload()),
		mkEntry(3, eventFill, fp),
	}
	r := reconcileEntries(entries)
	if r.Valid {
		t.Fatal("fill quantity mismatch not detected")
	}
	if rulesOf(r)["FILL_QTY_MISMATCH"] != 1 {
		t.Fatalf("wrong rules: %+v", r.Violations)
	}
}

func TestReconcile_EmptyHistoryIsValid(t *testing.T) {
	r := reconcileEntries(nil)
	if !r.Valid {
		t.Fatal("empty history should reconcile as valid")
	}
}

func TestReconcile_NonTradeEventsIgnored(t *testing.T) {
	entries := []AuditEntry{
		mkEntry(1, "control.kill", map[string]interface{}{"x": 1}),
		mkEntry(2, eventApprovalIssued, approvalPayload()),
		mkEntry(3, eventOrderSubmitted, orderPayload()),
		mkEntry(4, eventFill, fillPayload()),
	}
	r := reconcileEntries(entries)
	if !r.Valid {
		t.Fatalf("non-trade events broke reconciliation: %+v", r.Violations)
	}
}

// A retried approval append produces two aegis.approval_issued entries for
// the same approval_id (distinct entry ids). That must be counted and
// flagged — never silently overwritten.
func TestReconcile_DuplicateApprovalIssued(t *testing.T) {
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventApprovalIssued, approvalPayload()), // retry duplicate
		mkEntry(3, eventOrderSubmitted, orderPayload()),
		mkEntry(4, eventFill, fillPayload()),
	}
	r := reconcileEntries(entries)
	if r.Valid {
		t.Fatal("duplicate approval issuance not detected")
	}
	if rulesOf(r)["DUPLICATE_APPROVAL_ISSUED"] != 1 {
		t.Fatalf("wrong rules: %+v", r.Violations)
	}
	if r.ApprovalsIssued != 1 {
		t.Fatalf("distinct approval count = %d, want 1", r.ApprovalsIssued)
	}
	for _, v := range r.Violations {
		if v.Rule == "DUPLICATE_APPROVAL_ISSUED" && len(v.EntryIDs) != 2 {
			t.Fatalf("duplicate violation should cite both entries: %+v", v)
		}
	}
}

// A submitted order whose fill was never audited (and never acknowledged)
// is an unaudited trade: it must surface as a violation.
func TestReconcile_OrderWithoutFillFlagged(t *testing.T) {
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventOrderSubmitted, orderPayload()),
		// no fill, no gap record
	}
	r := reconcileEntries(entries)
	if r.Valid {
		t.Fatal("order without fill not detected")
	}
	if rulesOf(r)["ORDER_WITHOUT_FILL"] != 1 {
		t.Fatalf("wrong rules: %+v", r.Violations)
	}
}

// A submitted order covered by an explicit operator-acknowledged
// fill_audit_gap record is accounted for: no violation, but the gap stays
// counted in the report forever.
func TestReconcile_OrderWithGapRecordIsAccounted(t *testing.T) {
	gap := map[string]interface{}{
		"order_id": "o1", "approval_id": "a1",
		"acknowledged_by": "op-test", "gap_record_persisted": true,
	}
	entries := []AuditEntry{
		mkEntry(1, eventApprovalIssued, approvalPayload()),
		mkEntry(2, eventOrderSubmitted, orderPayload()),
		mkEntry(3, eventFillAuditGap, gap),
	}
	r := reconcileEntries(entries)
	if !r.Valid {
		t.Fatalf("gap-covered order reported violations: %+v", r.Violations)
	}
	if rulesOf(r)["ORDER_WITHOUT_FILL"] != 0 {
		t.Fatalf("gap-covered order flagged as missing fill: %+v", r.Violations)
	}
	if r.FillAuditGaps != 1 {
		t.Fatalf("FillAuditGaps = %d, want 1", r.FillAuditGaps)
	}
}
