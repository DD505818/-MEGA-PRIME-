// VULTURE Protocol — Execution Engine
// Implements the order FSM: NEW → RISK_PENDING → APPROVED → ROUTED →
// PARTIALLY_FILLED → FILLED (with REJECTED/CANCEL_PENDING/CANCELLED/FAILED).
package main

import (
	"context"
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
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
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
}

func NewExecutionEngine(redisAddr, brokers string) *ExecutionEngine {
	paperMode := os.Getenv("PAPER_MODE") == "" || os.Getenv("PAPER_MODE") == "true"
	if !paperMode || os.Getenv("LIVE_TRADING_ENABLED") == "true" {
		log.Fatal("LIVE execution is disabled pending broker certification and failure testing")
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
		redis:     rdb,
		consumer:  c,
		producer:  p,
		orders:    make(map[string]*Order),
		paperMode: paperMode,
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

func (e *ExecutionEngine) processSignal(signal map[string]interface{}) {
	order := e.signalToOrder(signal)

	// Pre-submit kill check (1B.2). The kill flag is durable state owned by
	// risk-service. A signal approved before a kill must not be submitted
	// after it — this closes the approve→submit race window. Reactive
	// cancellation of already-tracked orders still flows through
	// emergency.halt → handleHalt.
	if e.redis.Get(context.Background(), "kill_switch").Val() == "1" {
		e.mu.Lock()
		e.orders[order.ID] = order
		e.mu.Unlock()
		e.transition(order, StateCancelled)
		order.Meta["cancel_reason"] = "KILL_SWITCH_ACTIVE_AT_SUBMIT"
		log.Printf("ORDER %s: submission blocked — kill switch active at submit", order.ID[:8])
		return
	}

	e.mu.Lock()
	e.orders[order.ID] = order
	e.mu.Unlock()

	e.transition(order, StateRiskPending)

	// Risk approval already happened (signal came from signals.approved)
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

// liveFill sends order to broker (paper simulation delegates to paperFill in non-live).
func (e *ExecutionEngine) liveFill(order *Order, params map[string]interface{}) (float64, error) {
	switch order.Type {
	case TypeTWAP:
		return e.executeTWAP(order, params)
	case TypeIceberg:
		return e.executeIceberg(order, params)
	default:
		return e.paperFill(order) // broker integration point
	}
}

func (e *ExecutionEngine) executeTWAP(order *Order, params map[string]interface{}) (float64, error) {
	slices := 5
	if n, ok := params["n_slices"].(int); ok {
		slices = n
	}
	sliceQty := order.Qty / float64(slices)
	totalFill := 0.0
	for i := 0; i < slices; i++ {
		delayMs := 10 + rand.Intn(40)
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		fill, _ := e.paperFill(&Order{
			LimitPrice: order.LimitPrice,
			Qty:        sliceQty,
			Side:       order.Side,
		})
		totalFill += fill * sliceQty
	}
	return totalFill / order.Qty, nil
}

func (e *ExecutionEngine) executeIceberg(order *Order, params map[string]interface{}) (float64, error) {
	nSlices := 3
	if n, ok := params["n_slices"].(int); ok {
		nSlices = n
	}
	sliceQty := order.Qty / float64(nSlices)
	totalFill := 0.0
	for i := 0; i < nSlices; i++ {
		time.Sleep(30 * time.Millisecond)
		fill, _ := e.paperFill(&Order{
			LimitPrice: order.LimitPrice,
			Qty:        sliceQty,
			Side:       order.Side,
		})
		totalFill += fill * sliceQty
	}
	return totalFill / order.Qty, nil
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
