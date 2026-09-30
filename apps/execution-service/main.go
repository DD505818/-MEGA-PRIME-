// VULTURE Protocol — Execution Engine
// Implements the order FSM: NEW → RISK_PENDING → APPROVED → ROUTED →
// PARTIALLY_FILLED → FILLED (with REJECTED/CANCEL_PENDING/CANCELLED/FAILED).
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/omega-prime-delta/approval"
	"github.com/omega-prime-delta/modelock"
	"github.com/omega-prime-delta/truthclient"
)

// ── FSM States ───────────────────────────────────────────────────────────────

type OrderState string

const (
	StateNew             OrderState = "NEW"
	StateRiskPending     OrderState = "RISK_PENDING"
	StateApproved        OrderState = "APPROVED"
	StateRouted          OrderState = "ROUTED"
	StatePartiallyFilled OrderState = "PARTIALLY_FILLED"
	StateFilled          OrderState = "FILLED"
	StateRejected        OrderState = "REJECTED"
	StateCancelPending   OrderState = "CANCEL_PENDING"
	StateCancelled       OrderState = "CANCELLED"
	StateFailed          OrderState = "FAILED"
)

// ── Order Types ──────────────────────────────────────────────────────────────

type OrderType string

const (
	TypeMarket        OrderType = "MARKET"
	TypeLimit         OrderType = "LIMIT"
	TypeStop          OrderType = "STOP"
	TypeIceberg       OrderType = "ICEBERG"
	TypeTWAP          OrderType = "TWAP"
	TypeVWAP          OrderType = "VWAP"
	TypeAdaptiveLimit OrderType = "ADAPTIVE_LIMIT"
)

// ── Order ────────────────────────────────────────────────────────────────────

type Order struct {
	ID         string                 `json:"order_id"`
	SignalID   string                 `json:"signal_id"`
	StrategyID string                 `json:"strategy_id"`
	Symbol     string                 `json:"symbol"`
	Side       string                 `json:"side"`
	Qty        float64                `json:"quantity"`
	LimitPrice float64                `json:"limit_price"`
	StopPrice  float64                `json:"stop"`
	State      OrderState             `json:"state"`
	Type       OrderType              `json:"order_type"`
	Venue      string                 `json:"venue"`
	FilledQty  float64                `json:"filled_qty"`
	AvgFill    float64                `json:"avg_fill_price"`
	Slippage   float64                `json:"slippage_bps"`
	CreatedAt  int64                  `json:"created_at_ms"`
	UpdatedAt  int64                  `json:"updated_at_ms"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

// ── Execution Engine ─────────────────────────────────────────────────────────

type ExecutionEngine struct {
	redis    *redis.Client
	consumer *kafka.Consumer
	producer *kafka.Producer

	mu     sync.RWMutex
	orders map[string]*Order

	paperMode bool

	// approvalPub is the AEGIS public key for the authority boundary.
	// Every signal must carry a verifiable, single-use, unexpired approval
	// signed by the matching private key, or the order is refused.
	approvalPub ed25519.PublicKey

	// truthClient is the TruthCore audit client. Audit-or-no-trade: every
	// submitted order is appended to the hash chain BEFORE routing; an
	// append failure refuses the order (AUDIT_UNAVAILABLE). A nil client
	// fails closed — nothing is submitted.
	truthClient *truthclient.Client

	// auditHalted is set when a fill could not be recorded in TruthCore.
	// The fill already happened (paper simulation), so it cannot be
	// un-happened; instead all NEW submissions are refused until the audit
	// spine is reachable again. No new trade without its audit record.
	auditHalted atomic.Bool
}

func NewExecutionEngine(redisAddr, brokers string) *ExecutionEngine {
	// PAPER/LIVE lock (Phase 2 program): the process cannot start unless it
	// proves paper mode. LIVE is locked — no configuration can enable it.
	modelock.RequirePaper("execution-service")
	paperMode := modelock.IsPaper() // always true here; kept for submit-path clarity

	// Authority boundary: VULTURE cannot verify AEGIS approvals without the
	// public key, so it must not start without one.
	approvalPub, err := approval.ParsePublicKey(os.Getenv("AEGIS_APPROVAL_PUBKEY"))
	if err != nil {
		log.Fatalf("authority boundary: %v", err)
	}

	rdb := newRedisClient(redisAddr)

	c, err := kafka.NewConsumer(kafkaTransport(kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "execution-engine",
		"auto.offset.reset": "latest",
	}))
	if err != nil {
		log.Fatalf("execution kafka consumer: %v", err)
	}
	c.SubscribeTopics([]string{"signals.approved", "emergency.halt"}, nil)

	p, err := kafka.NewProducer(kafkaTransport(kafka.ConfigMap{"bootstrap.servers": brokers}))
	if err != nil {
		log.Fatalf("execution kafka producer: %v", err)
	}

	return &ExecutionEngine{
		redis:       rdb,
		consumer:    c,
		producer:    p,
		orders:      make(map[string]*Order),
		paperMode:   paperMode,
		approvalPub: approvalPub,
	}
}

func (e *ExecutionEngine) run() {
	log.Printf("VULTURE Protocol online (paper=%v)", e.paperMode)
	for {
		ev := e.consumer.Poll(100)
		if ev == nil {
			continue
		}
		switch msg := ev.(type) {
		case *kafka.Message:
			topic := *msg.TopicPartition.Topic
			switch topic {
			case "emergency.halt":
				e.handleHalt()
			case "signals.approved":
				var signal map[string]interface{}
				if err := json.Unmarshal(msg.Value, &signal); err == nil {
					go e.processSignal(signal)
				}
			}
		case kafka.Error:
			log.Printf("execution kafka error: %v", msg)
		}
	}
}

func (e *ExecutionEngine) handleHalt() {
	log.Println("HALT received — cancelling all orders")
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, o := range e.orders {
		if o.State == StateRouted || o.State == StatePartiallyFilled || o.State == StateApproved {
			e.transition(o, StateCancelPending)
			e.transition(o, StateCancelled)
		}
	}
	ctx := context.Background()
	e.redis.Set(ctx, "kill:confirmed", "1", 10*time.Second)
	log.Println("All orders cancelled — kill confirmed")
}

// verifyApproval enforces the AEGIS→VULTURE authority boundary on one
// signal. It returns the verified approval, or a cancel reason. Checks, in
// order: presence, signature, expiry, field binding against the signal, and
// atomic single-use claim in Redis (the database boundary — single-use is
// enforced by Redis atomicity, not by process memory).
func (e *ExecutionEngine) verifyApproval(signal map[string]interface{}) (*approval.Approval, string) {
	raw, ok := signal["aegis_approval"].(map[string]interface{})
	if !ok {
		return nil, "APPROVAL_MISSING"
	}
	appr, err := approval.FromMap(raw)
	if err != nil {
		return nil, "APPROVAL_MALFORMED"
	}
	if err := appr.Verify(e.approvalPub); err != nil {
		return nil, "APPROVAL_BAD_SIGNATURE"
	}
	now := time.Now()
	if appr.Expired(now) {
		return nil, "APPROVAL_EXPIRED"
	}
	// Field binding: the signal's execution-critical fields must equal the
	// approved values exactly. Any divergence means tampering or a stale
	// mix-up — refuse.
	qty, _ := toF64(signal["quantity"])
	limitPrice, _ := toF64(signal["limit_price"])
	stopPrice, _ := toF64(signal["stop"])
	bound := appr.SignalID == strOf(signal["signal_id"]) &&
		appr.StrategyID == strOf(signal["strategy_id"]) &&
		appr.Symbol == strOf(signal["symbol"]) &&
		appr.Side == strOf(signal["side"]) &&
		appr.Quantity == qty &&
		appr.LimitPrice == limitPrice &&
		appr.StopPrice == stopPrice &&
		appr.Mode == strOf(signal["mode"])
	if !bound {
		return nil, "APPROVAL_FIELD_MISMATCH"
	}
	// Single-use claim: atomic SET NX. If the key already exists the
	// approval was consumed — a replay.
	claimed, err := e.redis.SetNX(context.Background(), appr.ClaimKey(), "claimed", approval.ClaimKeyTTL).Result()
	if err != nil {
		return nil, "APPROVAL_CLAIM_STORE_UNAVAILABLE"
	}
	if !claimed {
		return nil, "APPROVAL_REPLAY"
	}
	return appr, ""
}

// killActiveAtSubmit reads the durable kill flag owned by risk-service.
// Fail closed: a Redis error means the kill state is UNKNOWN, which must
// block submission, not permit it (the old .Val() call failed open).
func (e *ExecutionEngine) killActiveAtSubmit() (active, unknown bool) {
	killVal, err := e.redis.Get(context.Background(), "kill_switch").Result()
	if err != nil && err != redis.Nil {
		return false, true
	}
	return killVal == "1", false
}

// recordOrderSubmitted appends the order to the TruthCore hash chain.
// Blocking: an error refuses the order (audit-or-no-trade).
func (e *ExecutionEngine) recordOrderSubmitted(order *Order, appr *approval.Approval) error {
	if e.truthClient == nil {
		return fmt.Errorf("truth client not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	payload := map[string]interface{}{
		"order_id":     order.ID,
		"signal_id":    order.SignalID,
		"strategy_id":  order.StrategyID,
		"approval_id":  appr.ApprovalID,
		"symbol":       order.Symbol,
		"side":         order.Side,
		"quantity":     order.Qty,
		"limit_price":  order.LimitPrice,
		"stop_price":   order.StopPrice,
		"mode":         appr.Mode,
		"submitted_at_ms": time.Now().UnixMilli(),
	}
	_, err := e.truthClient.Append(ctx, truthclient.EventOrderSubmitted, payload)
	return err
}

// recordFill appends the fill to the TruthCore hash chain. A failure is
// reported to the caller, which marks the audit gap and halts new
// submissions — the fill cannot be un-happened.
func (e *ExecutionEngine) recordFill(order *Order) error {
	if e.truthClient == nil {
		return fmt.Errorf("truth client not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	approvalID, _ := order.Meta["approval_id"].(string)
	payload := map[string]interface{}{
		"order_id":     order.ID,
		"signal_id":    order.SignalID,
		"strategy_id":  order.StrategyID,
		"approval_id":  approvalID,
		"symbol":       order.Symbol,
		"side":         order.Side,
		"quantity":     order.FilledQty,
		"fill_price":   order.AvgFill,
		"filled_at_ms": time.Now().UnixMilli(),
	}
	_, err := e.truthClient.Append(ctx, truthclient.EventFill, payload)
	return err
}

// recordRefusal appends a refusal to the hash chain. Best-effort: the
// refusal is already the safe outcome; a failed audit write is only logged.
func (e *ExecutionEngine) recordRefusal(signal map[string]interface{}, reason string) {
	if e.truthClient == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	var approvalID string
	if appr, ok := signal["aegis_approval"].(map[string]interface{}); ok {
		approvalID, _ = appr["approval_id"].(string)
	}
	payload := map[string]interface{}{
		"signal_id":     strOf(signal["signal_id"]),
		"strategy_id":   strOf(signal["strategy_id"]),
		"approval_id":   approvalID,
		"reason":        reason,
		"refused_at_ms": time.Now().UnixMilli(),
	}
	if _, err := e.truthClient.Append(ctx, truthclient.EventOrderRefused, payload); err != nil {
		log.Printf("refusal audit failed (non-fatal): %v", err)
	}
}

// refuse cancels an order before submission with an audited reason.
func (e *ExecutionEngine) refuse(signal map[string]interface{}, reason string) {
	e.recordRefusal(signal, reason)
	order := e.signalToOrder(signal)
	e.mu.Lock()
	e.orders[order.ID] = order
	e.mu.Unlock()
	e.transition(order, StateCancelled)
	order.Meta["cancel_reason"] = reason
	if appr, ok := signal["aegis_approval"].(map[string]interface{}); ok {
		if id, ok := appr["approval_id"].(string); ok {
			order.Meta["approval_id"] = id
		}
	}
	log.Printf("ORDER %s: submission refused — %s", order.ID[:8], reason)
}

func (e *ExecutionEngine) processSignal(signal map[string]interface{}) {
	order := e.signalToOrder(signal)

	// PAPER/LIVE lock: re-assert paper mode on every worker message, so a
	// mode change after startup can never leak an order toward a live venue.
	if err := modelock.AssertPaper(); err != nil {
		e.refuse(signal, "MODELOCK_REFUSAL")
		return
	}

	// Authority boundary: only a signed, unexpired, single-use AEGIS
	// approval authorizes submission. This is checked independently at
	// VULTURE — trust in the topic is not enough.
	appr, reason := e.verifyApproval(signal)
	if reason != "" {
		e.refuse(signal, reason)
		return
	}

	// Pre-submit kill check (1B.2). The kill flag is durable state owned by
	// risk-service. A signal approved before a kill must not be submitted
	// after it — this closes the approve→submit race window. Reactive
	// cancellation of already-tracked orders still flows through
	// emergency.halt → handleHalt.
	// Fail closed: a Redis error means the kill state is UNKNOWN, which
	// must block submission, not permit it.
	killActive, killUnknown := e.killActiveAtSubmit()
	if killUnknown {
		e.refuse(signal, "KILL_SWITCH_STATE_UNKNOWN")
		return
	}
	if killActive {
		e.refuse(signal, "KILL_SWITCH_ACTIVE_AT_SUBMIT")
		return
	}

	// Audit-or-no-trade: the order is durably recorded in the TruthCore
	// hash chain BEFORE any routing or state. If the audit spine is down,
	// the order is refused — no trade may exist without its audit record.
	// A prior unaudited fill also halts all new submissions.
	if e.auditHalted.Load() {
		e.refuse(signal, "AUDIT_HALTED")
		return
	}
	order.Meta["approval_id"] = appr.ApprovalID
	order.Meta["aegis_gates_version"] = appr.GatesVersion
	if err := e.recordOrderSubmitted(order, appr); err != nil {
		log.Printf("ORDER %s: truth-core append failed: %v", order.ID[:8], err)
		e.refuse(signal, "AUDIT_UNAVAILABLE")
		return
	}

	e.mu.Lock()
	e.orders[order.ID] = order
	e.mu.Unlock()

	e.transition(order, StateRiskPending)

	// AEGIS approval verified above (signature, expiry, binding, single-use).
	e.transition(order, StateApproved)

	// Select algorithm and venue
	algoType, algoParams := selectAlgo(order)
	order.Type = algoType
	order.Venue = selectVenue(order.Symbol)

	e.transition(order, StateRouted)
	e.publish("orders.routed", order)

	// Execute based on algorithm
	var fillPrice float64
	var err error

	if e.paperMode {
		fillPrice, err = e.paperFill(order)
	} else {
		fillPrice, err = e.liveFill(order, algoParams)
	}

	if err != nil {
		e.transition(order, StateFailed)
		order.Meta["error"] = err.Error()
	} else if fillPrice > 0 {
		order.AvgFill = fillPrice
		order.FilledQty = order.Qty
		order.Slippage = ((fillPrice - order.LimitPrice) / order.LimitPrice) * 10_000
		e.transition(order, StateFilled)
		// The fill is durably recorded in the hash chain. If the append
		// fails the fill cannot be un-happened, so the order is marked
		// audit_gap and ALL new submissions halt until the audit spine
		// recovers — audit-or-no-trade.
		if err := e.recordFill(order); err != nil {
			log.Printf("ORDER %s: FILL NOT AUDITED: %v — halting new submissions", order.ID[:8], err)
			order.Meta["audit_gap"] = true
			order.Meta["audit_gap_error"] = err.Error()
			e.auditHalted.Store(true)
			// Structured critical log: the durable record of the gap.
			// (execution-service has no alert topic; the halt is enforced
			// in-process via auditHalted and visible in order Meta.)
			log.Printf("CRITICAL FILL_AUDIT_GAP order_id=%s approval_id=%s error=%q",
				order.ID, order.Meta["approval_id"], err.Error())
		}
	}

	e.publish("orders.fills", order)
	e.updatePortfolio(order)
}

// paperFill simulates a fill with realistic slippage.
func (e *ExecutionEngine) paperFill(order *Order) (float64, error) {
	// Simulate market impact: σ * sqrt(Q / ADV)
	// Use a simplified model: 0.0001 * sqrt(notional / 100_000)
	notional := order.LimitPrice * order.Qty
	impact := 0.0001 * math.Sqrt(notional/100_000)
	jitter := (rand.Float64() - 0.5) * 0.0002
	if os.Getenv("PAPER_DETERMINISTIC") == "true" {
		h := fnv.New64a()
		_, _ = h.Write([]byte(order.SignalID))
		jitter = (float64(h.Sum64()%20_001) / 100_000_000.0) - 0.0001
	}

	if order.Side == "BUY" {
		return order.LimitPrice * (1 + impact + jitter), nil
	}
	return order.LimitPrice * (1 - impact + jitter), nil
}

// liveFill is the future broker integration point. LIVE is locked: this
// function must never execute a real order. It returns a hard error instead
// of silently simulating, so any accidental routing to it is loud and the
// order fails instead of filling against a phantom venue.
func (e *ExecutionEngine) liveFill(order *Order, params map[string]interface{}) (float64, error) {
	return 0, fmt.Errorf("modelock: LIVE execution is locked — no broker integration exists and none may be added without certification")
}

func (e *ExecutionEngine) signalToOrder(signal map[string]interface{}) *Order {
	now := time.Now().UnixMilli()
	price, _ := toF64(signal["limit_price"])
	qty, _ := toF64(signal["quantity"])
	stop, _ := toF64(signal["stop"])
	return &Order{
		ID:         uuid.NewString(),
		SignalID:   strOf(signal["signal_id"]),
		StrategyID: strOf(signal["strategy_id"]),
		Symbol:     strOf(signal["symbol"]),
		Side:       strOf(signal["side"]),
		Qty:        qty,
		LimitPrice: price,
		StopPrice:  stop,
		State:      StateNew,
		CreatedAt:  now,
		UpdatedAt:  now,
		Meta:       make(map[string]interface{}),
	}
}

func (e *ExecutionEngine) transition(order *Order, state OrderState) {
	log.Printf("ORDER %s: %s → %s", order.ID[:8], order.State, state)
	order.State = state
	order.UpdatedAt = time.Now().UnixMilli()
}

func (e *ExecutionEngine) publish(topic string, order *Order) {
	if e.producer == nil {
		return
	}
	msg, _ := json.Marshal(order)
	_ = e.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Value: msg,
	}, nil)
}

func (e *ExecutionEngine) updatePortfolio(order *Order) {
	if order.State != StateFilled {
		return
	}
	ctx := context.Background()
	symbol := order.Symbol
	key := fmt.Sprintf("portfolio:position:%s", symbol)

	// 1. Append to the immutable fills ledger FIRST. The 60s reconciler in
	// risk-service recomputes expected positions from this ledger, so a
	// crash between this write and the position update below surfaces as a
	// divergence (fail-closed: kill) rather than silent drift.
	fillRec, _ := json.Marshal(map[string]interface{}{
		"order_id":  order.ID,
		"signal_id": order.SignalID,
		"symbol":    symbol,
		"side":      order.Side,
		"qty":       order.FilledQty,
		"ts":        time.Now().UnixMilli(),
	})
	ledgerKey := fmt.Sprintf("portfolio:fills:%s", symbol)
	e.redis.LPush(ctx, ledgerKey, fillRec)
	e.redis.LTrim(ctx, ledgerKey, 0, 9999)
	e.redis.SAdd(ctx, "portfolio:tracked_symbols", symbol)

	// 2. Update the net position quantity.
	delta := order.FilledQty
	if order.Side != "BUY" {
		delta = -delta
	}
	newQty, err := e.redis.IncrByFloat(ctx, key, delta).Result()
	if err != nil {
		log.Printf("portfolio position update failed for %s: %v", symbol, err)
		return
	}

	// 3. Maintain the open-symbols set idempotently. SADD/SREM have no
	// counter to drift: the set always reflects "symbols with nonzero net
	// position", and a close (qty back to ~0) removes the symbol on the same
	// fill event that zeroed it. This replaces the old
	// portfolio:open_positions counter, which was incremented on every fill
	// and never decremented (and fought portfolio-service's SET overwrite
	// of the same key with different semantics).
	if isOpenPosition(newQty) {
		e.redis.SAdd(ctx, "portfolio:open_symbols", symbol)
	} else {
		e.redis.SRem(ctx, "portfolio:open_symbols", symbol)
	}
}

// positionQtyEpsilon treats dust quantities as a closed position.
const positionQtyEpsilon = 1e-9

// isOpenPosition reports whether a net quantity counts as an open position.
func isOpenPosition(qty float64) bool {
	return math.Abs(qty) > positionQtyEpsilon
}

// ── Algo Selection ────────────────────────────────────────────────────────────

func selectAlgo(order *Order) (OrderType, map[string]interface{}) {
	notional := order.LimitPrice * order.Qty
	switch {
	case notional > 30_000:
		return TypeVWAP, map[string]interface{}{"participation_rate": 0.1}
	case notional > 10_000:
		nSlices := min3(5, max2(3, int(notional/4_000)), 5)
		return TypeIceberg, map[string]interface{}{"n_slices": nSlices, "interval_secs": 30}
	case notional > 2_000:
		return TypeTWAP, map[string]interface{}{"duration_mins": 10, "n_slices": 5}
	default:
		return TypeLimit, map[string]interface{}{}
	}
}

func selectVenue(symbol string) string {
	switch {
	case len(symbol) > 3 && (symbol[len(symbol)-4:] == "USDT" || symbol[len(symbol)-3:] == "BTC"):
		return "Binance"
	case symbol == "XAUUSD" || symbol == "EURUSD":
		return "OANDA"
	case symbol == "ES" || symbol == "NQ" || symbol == "GC":
		return "IBKR"
	default:
		return "Alpaca"
	}
}

// ── HTTP ─────────────────────────────────────────────────────────────────────

func (e *ExecutionEngine) ordersHandler(w http.ResponseWriter, r *http.Request) {
	e.mu.RLock()
	orders := make([]*Order, 0, len(e.orders))
	for _, o := range e.orders {
		orders = append(orders, o)
	}
	e.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func trimRedis(addr string) string {
	if len(addr) > 8 && addr[:8] == "redis://" {
		return addr[8:]
	}
	return addr
}

func toF64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

func strOf(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	eng := NewExecutionEngine(
		os.Getenv("REDIS_URL"),
		os.Getenv("KAFKA_BROKERS"),
	)
	// TruthCore audit client (Phase 4, audit-or-no-trade). Writes are
	// authenticated when TRUTHCORE_WRITE_SECRET is set.
	truthURL := os.Getenv("TRUTHCORE_URL")
	if truthURL == "" {
		truthURL = "http://truth-core:8084"
	}
	eng.truthClient = truthclient.New(truthURL, os.Getenv("TRUTHCORE_WRITE_SECRET"))
	go eng.run()

	mux := http.NewServeMux()
	liveHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"live"}`))
	}
	readyHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := eng.redis.Ping(ctx).Err(); err != nil {
			http.Error(w, `{"status":"not_ready","dependency":"redis"}`, http.StatusServiceUnavailable)
			return
		}
		if _, err := eng.producer.GetMetadata(nil, true, 2000); err != nil {
			http.Error(w, `{"status":"not_ready","dependency":"kafka"}`, http.StatusServiceUnavailable)
			return
		}
		if eng.truthClient == nil || eng.truthClient.Ready(ctx) != nil {
			http.Error(w, `{"status":"not_ready","dependency":"truth_core"}`, http.StatusServiceUnavailable)
			return
		}
		assignment, err := eng.consumer.Assignment()
		if err != nil || len(assignment) == 0 {
			http.Error(w, `{"status":"not_ready","dependency":"kafka_consumer"}`, http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
	mux.HandleFunc("/health", liveHandler)
	mux.HandleFunc("/health/live", liveHandler)
	mux.HandleFunc("/health/ready", readyHandler)
	mux.HandleFunc("/orders", eng.ordersHandler)

	go func() {
		log.Println("Execution service HTTP on :8081")
		http.ListenAndServe(":8081", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
