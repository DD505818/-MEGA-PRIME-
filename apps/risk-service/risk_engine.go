package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/omega-prime-delta/modelock"
)

// RiskEngine implements the 14-gate AEGIS Governor.
type RiskEngine struct {
	redis        redisStore
	consumer     *kafka.Consumer
	producer     *kafka.Producer
	killSwitch   atomic.Bool
	circuitBreak atomic.Bool

	// controlMu serializes kill/reset state transitions so concurrent
	// /kill and /reset calls cannot interleave: the in-memory flag and the
	// durable Redis flag are always written under this lock, so they cannot
	// diverge. controlSeq is a monotonic sequence (backed by Redis INCR)
	// that orders transitions for audit and post-incident review.
	controlMu  sync.Mutex
	controlSeq atomic.Int64

	// stateVerified is false when the process booted without successfully
	// reading the durable kill state (Redis unavailable). Until verified,
	// the switch is assumed KILLED (fail closed); see restoreControlState.
	stateVerified atomic.Bool

	maxDailyLoss     float64
	maxDrawdown      float64
	maxPositions     int
	maxNotional      float64
	maxLeverage      float64
	maxSpreadBps     float64
	riskPerTrade     float64
	minConfidence    float64
	staleBookSecs    int64
	dedupTTL         time.Duration
	maxAssetExposure float64
	maxCorrelation   float64
}

func NewRiskEngine(redisAddr, brokers string) *RiskEngine {
	rdb := newRedisClient(redisAddr)

	c, err := kafka.NewConsumer(kafkaTransport(kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "risk-engine",
		"auto.offset.reset": "latest",
	}))
	if err != nil {
		log.Fatalf("kafka consumer: %v", err)
	}
	// 1B.4 topology: risk validates the FINAL signal — fused by
	// fusion-engine and sized by capital-allocator. Subscribing to
	// signals.sized (not signals.raw) is what eliminates the allocation
	// race: Gate 14's notional/leverage/risk-per-trade checks run on the
	// exact quantity execution-service will trade. See
	// docs/signal-topology.md.
	c.SubscribeTopics([]string{"signals.sized"}, nil)

	p, err := kafka.NewProducer(kafkaTransport(kafka.ConfigMap{"bootstrap.servers": brokers}))
	if err != nil {
		log.Fatalf("kafka producer: %v", err)
	}

	return &RiskEngine{
		redis:            rdb,
		consumer:         c,
		producer:         p,
		maxDailyLoss:     envFloat("MAX_DAILY_LOSS", 0.02),
		maxDrawdown:      envFloat("MAX_DRAWDOWN", 0.10),
		maxPositions:     envInt("MAX_POSITIONS", 8),
		maxNotional:      envFloat("MAX_NOTIONAL_PER_TRADE", 50_000),
		maxLeverage:      envFloat("MAX_LEVERAGE", 2.0),
		maxSpreadBps:     envFloat("MAX_SPREAD_BPS", 20),
		riskPerTrade:     envFloat("RISK_PER_TRADE", 0.005),
		minConfidence:    envFloat("MIN_CONFIDENCE", 0.60),
		staleBookSecs:    int64(envInt("STALE_BOOK_SECONDS", 5)),
		dedupTTL:         time.Duration(envInt("DEDUP_TTL_SECONDS", 86400)) * time.Second,
		maxAssetExposure: envFloat("MAX_ASSET_EXPOSURE_PCT", 0.25),
		maxCorrelation:   envFloat("MAX_PAIR_CORRELATION", 0.70),
	}
}

// validate runs all 14 gates and returns (approved bool, reason string, adjusted_qty float64).
func (r *RiskEngine) validate(signal map[string]interface{}) (bool, string, float64) {
	ctx := context.Background()

	// ── Gate 1: Kill Switch ─────────────────────────────────────────────────
	if r.killSwitch.Load() {
		return false, "GATE1_KILL_SWITCH_ACTIVE", 0
	}

	// ── Gate 2: Circuit Breaker ─────────────────────────────────────────────
	if r.circuitBreak.Load() {
		return false, "GATE2_CIRCUIT_BREAKER_ACTIVE", 0
	}

	equity := r.redisFloat(ctx, "portfolio:equity")
	if equity <= 0 {
		equity = 100_000 // safe default if not yet set
	}

	// ── Gate 3: Paper/Live Mode Mismatch ────────────────────────────────────
	// Canonical mode comes from modelock: the process is locked to paper at
	// startup (RequirePaper in main). This gate additionally rejects any
	// signal declaring a non-paper mode, and refuses validation entirely if
	// the process is somehow not in paper mode. Ambiguous mode → reject.
	signalMode, _ := signal["mode"].(string)
	if !modelock.IsPaper() {
		return false, "GATE3_NOT_IN_PAPER_MODE", 0
	}
	if signalMode != "" && !strings.EqualFold(strings.TrimSpace(signalMode), "paper") {
		return false, "GATE3_SIGNAL_MODE_MISMATCH", 0
	}

	// ── Gate 4: Broker Health ───────────────────────────────────────────────
	brokerStatus := r.redisString(ctx, "broker:status")
	if brokerStatus == "DOWN" || brokerStatus == "DEGRADED" {
		return false, "GATE4_BROKER_DOWN", 0
	}

	// ── Gate 5: Stale Market Data (FAIL-CLOSED) ─────────────────────────────
	// Freshness key: book_ts:<symbol> — millis epoch of the last book/price
	// update for the symbol. Canonical contract: the market-data pipeline
	// (wired in 1B.4) must SET this key on every book update.
	//
	// A missing, unparseable, zero, future, or stale timestamp REJECTS the
	// signal. There is no "no data, assume fresh" path: a stale price feeding
	// the risk engine is a direct path to a bad fill. The fail-closed action
	// is refusing new intents at this gate (it does not flatten — flattening
	// is the kill switch's job). Every fail-closed event is audited with the
	// same three-layer discipline as the control plane (auditFreshnessFailClosed),
	// no exceptions and no aggregation.
	symbol, _ := signal["symbol"].(string)
	bookTSKey := fmt.Sprintf("book_ts:%s", symbol)
	bookTSStr := r.redisString(ctx, bookTSKey)
	maxAgeMs := r.staleBookMs(symbol)
	nowMs := time.Now().UnixMilli()
	failReason := ""
	switch {
	case bookTSStr == "":
		failReason = "GATE5_NO_BOOK_TS"
	default:
		bookTS, perr := strconv.ParseInt(strings.TrimSpace(bookTSStr), 10, 64)
		switch {
		case perr != nil:
			failReason = "GATE5_BAD_BOOK_TS"
		case bookTS <= 0:
			// Synthetic feeds publish timestamp: 0. A zero timestamp is not
			// "very old data" — it is no data, and it fails closed.
			failReason = "GATE5_ZERO_BOOK_TS"
		case bookTS-nowMs > 60_000:
			failReason = fmt.Sprintf("GATE5_FUTURE_BOOK_TS_%dms", bookTS-nowMs)
		case nowMs-bookTS > maxAgeMs:
			failReason = fmt.Sprintf("GATE5_STALE_BOOK_%dms", nowMs-bookTS)
		}
	}
	if failReason != "" {
		r.auditFreshnessFailClosed(symbol, failReason)
		return false, failReason, 0
	}

	// ── Gate 6: Daily Loss Cap ──────────────────────────────────────────────
	dailyPnL := r.redisFloat(ctx, "portfolio:daily_pnl")
	if equity > 0 && dailyPnL/equity <= -r.maxDailyLoss {
		r.triggerCircuitBreaker("GATE6_DAILY_LOSS_EXCEEDED")
		return false, "GATE6_DAILY_LOSS_EXCEEDED", 0
	}

	// ── Gate 7: Max Drawdown ────────────────────────────────────────────────
	peakEquity := r.redisFloat(ctx, "portfolio:peak_equity")
	if peakEquity <= 0 {
		peakEquity = equity
	}
	drawdown := (peakEquity - equity) / peakEquity
	if drawdown >= r.maxDrawdown {
		r.activateKillSwitch("GATE7_MAX_DRAWDOWN_BREACH")
		return false, "GATE7_MAX_DRAWDOWN_EXCEEDED", 0
	}
	// Circuit breaker cascade
	r.checkCircuitBreakerCascade(drawdown, equity)

	// ── Gate 8: Max Open Positions ──────────────────────────────────────────
	// The count is SCARD of the portfolio:open_symbols set — the symbols
	// with nonzero net position — maintained idempotently by
	// execution-service on fill events. (1B.2: the old
	// portfolio:open_positions counter was incremented on every fill and
	// never decremented, while portfolio-service overwrote the same key
	// with its own len() — two writers, two semantics, guaranteed drift.)
	openPos := int(r.redis.SCard(ctx, "portfolio:open_symbols").Val())
	if openPos >= r.maxPositions {
		return false, "GATE8_MAX_POSITIONS_REACHED", 0
	}

	// ── Gate 9: Asset Exposure Cap ──────────────────────────────────────────
	assetExposureKey := fmt.Sprintf("portfolio:exposure:%s", symbol)
	assetExposure := r.redisFloat(ctx, assetExposureKey)
	if equity > 0 && assetExposure/equity > r.maxAssetExposure {
		return false, fmt.Sprintf("GATE9_ASSET_EXPOSURE_%.1f%%", assetExposure/equity*100), 0
	}

	// ── Gate 10: Correlation Guard ──────────────────────────────────────────
	maxCorr := r.redisFloat(ctx, fmt.Sprintf("portfolio:max_corr:%s", symbol))
	if maxCorr > r.maxCorrelation {
		return false, fmt.Sprintf("GATE10_HIGH_CORRELATION_%.2f", maxCorr), 0
	}

	// ── Gate 11: Duplicate Signal ID ────────────────────────────────────────
	// Durable atomic check-and-set (1B.3). SET NX is a single atomic op:
	// the dedup record is written to durable Redis BEFORE the approved
	// signal is published to signals.approved, so a crash between submit
	// and record cannot produce a duplicate order — the redelivered signal
	// finds the record and is rejected. No check-then-set race between
	// replicas, no in-memory map to lose on restart (the old 50k-entry
	// wipe could re-admit old IDs). The TTL bounds memory; it must exceed
	// the longest window in which a redelivered duplicate could arrive
	// (Kafka rebalance/redelivery + strategy re-emit cadence). Default 24h
	// via DEDUP_TTL_SECONDS.
	// Fail-closed: if the dedup store is unavailable we cannot prove
	// uniqueness, so the signal is rejected. A dropped signal is a missed
	// trade; a duplicated signal is an unintended position.
	signalID, _ := signal["signal_id"].(string)
	dedupKey := "risk:seen_signal:" + signalID
	added, err := r.redis.SetNX(ctx, dedupKey, "1", r.dedupTTL).Result()
	if err != nil {
		return false, "GATE11_DEDUP_STORE_UNAVAILABLE", 0
	}
	if !added {
		return false, "GATE11_DUPLICATE_SIGNAL_ID", 0
	}

	// ── Gate 12: Spread Guard ───────────────────────────────────────────────
	spreadBps := r.redisFloat(ctx, fmt.Sprintf("book_spread:%s", symbol))
	if spreadBps > 0 && spreadBps > r.maxSpreadBps {
		return false, fmt.Sprintf("GATE12_SPREAD_TOO_WIDE_%.1fbps", spreadBps), 0
	}

	// ── Gate 13: Minimum Confidence ─────────────────────────────────────────
	confidence, _ := toFloat64(signal["confidence"])
	if confidence < r.minConfidence {
		return false, fmt.Sprintf("GATE13_LOW_CONFIDENCE_%.2f", confidence), 0
	}

	// ── Gate 14: Risk Per Trade ─────────────────────────────────────────────
	price, _ := toFloat64(signal["limit_price"])
	qty, _ := toFloat64(signal["quantity"])
	stop, _ := toFloat64(signal["stop"])

	if price <= 0 {
		return false, "GATE14_INVALID_PRICE", 0
	}

	// Notional cap
	notional := price * qty
	if notional > r.maxNotional {
		// Clip qty to fit within notional cap
		qty = r.maxNotional / price
	}

	// Leverage check
	if equity > 0 && notional/equity > r.maxLeverage {
		qty = (r.maxLeverage * equity) / price
	}

	// Risk per trade: |entry - stop| * qty <= riskPerTrade * equity
	if stop > 0 && price > 0 {
		riskDist := abs(price - stop)
		if riskDist > 0 {
			maxQty := (r.riskPerTrade * equity) / riskDist
			if qty > maxQty {
				qty = maxQty
			}
		}
	}

	if qty <= 0 {
		return false, "GATE14_QTY_REDUCED_TO_ZERO", 0
	}

	return true, "APPROVED", qty
}

// staleBookMs returns the maximum acceptable book age in milliseconds.
// Default from STALE_BOOK_SECONDS (5s); per-symbol override via
// STALE_BOOK_SECONDS_<SYMBOL> (e.g. STALE_BOOK_SECONDS_BTCUSDT=10) for
// assets whose quote cadence legitimately differs.
func (r *RiskEngine) staleBookMs(symbol string) int64 {
	if v := os.Getenv("STALE_BOOK_SECONDS_" + symbol); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return int64(n) * 1000
		}
	}
	if r.staleBookSecs > 0 {
		return r.staleBookSecs * 1000
	}
	return 5 * 1000
}

// auditFreshnessFailClosed records EVERY Gate 5 fail-closed rejection with
// the same three-layer discipline as control-plane actions (stdout, Redis
// stream, TruthCore with gap records). No cooldown, no aggregation: the
// standing rule is that every fail-closed event is audited, no exceptions.
// (Post-1B.4, signals reaching risk are fused consensus events — low
// volume — so per-event auditing does not flood the stream.)
func (r *RiskEngine) auditFreshnessFailClosed(symbol, reason string) {
	r.recordControlAudit("risk.gate5", "risk-engine", true, "fail_closed",
		fmt.Sprintf("symbol=%s reason=%s", symbol, reason))
}

// checkCircuitBreakerCascade implements the three-level cascade.
func (r *RiskEngine) checkCircuitBreakerCascade(drawdown, equity float64) {
	ctx := context.Background()
	switch {
	case drawdown >= 0.10:
		r.activateKillSwitch("CASCADE_L3_KILL")
	case drawdown >= 0.07:
		r.circuitBreak.Store(true)
		r.publishAlert(ctx, "CASCADE_L2_RESTRICT", map[string]interface{}{
			"action": "close_50pct_halt_new", "drawdown": drawdown,
		})
	case drawdown >= 0.05:
		r.publishAlert(ctx, "CASCADE_L1_WARNING", map[string]interface{}{
			"action": "reduce_sizes_25pct", "drawdown": drawdown,
		})
	}
}

func (r *RiskEngine) activateKillSwitch(reason string) (alreadyActive bool, err error) {
	r.controlMu.Lock()
	defer r.controlMu.Unlock()
	if r.killSwitch.Load() {
		return true, nil // idempotent: already killed
	}
	ctx := context.Background()
	seq, serr := r.nextControlSeq(ctx)
	if serr != nil {
		return false, fmt.Errorf("control seq: %w", serr)
	}
	// Durable write FIRST. If it fails, the in-memory flag is never set:
	// a failed kill must never look successful.
	if err := r.redis.Set(ctx, "kill_switch", "1", 0).Err(); err != nil {
		return false, fmt.Errorf("durable kill write: %w", err)
	}
	r.killSwitch.Store(true)
	r.controlSeq.Store(seq)
	log.Printf("KILL SWITCH ACTIVATED: %s (seq=%d)", reason, seq)
	r.publishHalt(ctx, reason)
	// Schedule kill confirmation check after 5s
	go r.confirmKillCascade(reason)
	return false, nil
}

// resetKillSwitch clears the kill state. Requires the caller to hold control
// authority (the /reset endpoint enforces admin role). Crash-safe order:
// memory first, then the durable delete, so a mid-transition crash boots back
// into KILLED (fail closed). A failed durable delete rolls the in-memory
// flags back instead of reporting a reset that did not persist.
func (r *RiskEngine) resetKillSwitch() error {
	r.controlMu.Lock()
	defer r.controlMu.Unlock()
	ctx := context.Background()
	seq, err := r.nextControlSeq(ctx)
	if err != nil {
		return fmt.Errorf("control seq: %w", err)
	}
	prevKill, prevCircuit := r.killSwitch.Load(), r.circuitBreak.Load()
	r.killSwitch.Store(false)
	r.circuitBreak.Store(false)
	if err := r.redis.Del(ctx, "kill_switch").Err(); err != nil {
		r.killSwitch.Store(prevKill)
		r.circuitBreak.Store(prevCircuit)
		return fmt.Errorf("durable reset: %w", err)
	}
	r.controlSeq.Store(seq)
	return nil
}

// nextControlSeq returns the next monotonic control-plane sequence number.
// Must be called with controlMu held.
func (r *RiskEngine) nextControlSeq(ctx context.Context) (int64, error) {
	return r.redis.Incr(ctx, controlSeqKey).Result()
}

func (r *RiskEngine) confirmKillCascade(reason string) {
	time.Sleep(5 * time.Second)
	ctx := context.Background()
	confirmed := r.redis.Get(ctx, "kill:confirmed").Val()
	if confirmed != "1" {
		log.Printf("KILL CASCADE ESCALATION: broker orders not confirmed cancelled — %s", reason)
		r.publishHalt(ctx, "KILL_ESCALATION_"+reason)
	}
}

func (r *RiskEngine) triggerCircuitBreaker(reason string) {
	if r.circuitBreak.CompareAndSwap(false, true) {
		log.Printf("Circuit breaker tripped: %s", reason)
	}
}

func (r *RiskEngine) publishHalt(ctx context.Context, reason string) {
	if r.producer == nil {
		return
	}
	topic := "emergency.halt"
	msg, _ := json.Marshal(map[string]interface{}{
		"reason": reason, "ts": time.Now().UnixMilli(),
	})
	_ = r.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg,
	}, nil)
}

func (r *RiskEngine) publishAlert(ctx context.Context, level string, data map[string]interface{}) {
	if r.producer == nil {
		return
	}
	topic := "risk.alerts"
	data["level"] = level
	data["ts"] = time.Now().UnixMilli()
	msg, _ := json.Marshal(data)
	_ = r.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg,
	}, nil)
}

func (r *RiskEngine) run() {
	log.Println("AEGIS Governor online — 14 gates active")
	for {
		ev := r.consumer.Poll(100)
		if ev == nil {
			continue
		}
		switch e := ev.(type) {
		case *kafka.Message:
			var signal map[string]interface{}
			if err := json.Unmarshal(e.Value, &signal); err != nil {
				continue
			}
			approved, reason, adjQty := r.validate(signal)
			if approved {
				signal["quantity"] = adjQty
				signal["risk_approved"] = true
				r.forward("signals.approved", signal)
			} else {
				signal["reject_reason"] = reason
				signal["risk_approved"] = false
				r.forward("signals.rejected", signal)
				log.Printf("REJECT [%s] %s → %s", signal["strategy_id"], signal["signal_id"], reason)
			}
		case kafka.Error:
			log.Printf("kafka error: %v", e)
		}
	}
}

func (r *RiskEngine) forward(topic string, signal map[string]interface{}) {
	if r.producer == nil {
		return
	}
	msg, _ := json.Marshal(signal)
	_ = r.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          msg,
	}, nil)
}

func (r *RiskEngine) redisFloat(ctx context.Context, key string) float64 {
	v, err := r.redis.Get(ctx, key).Float64()
	if err != nil {
		return 0
	}
	return v
}

func (r *RiskEngine) redisString(ctx context.Context, key string) string {
	return r.redis.Get(ctx, key).Val()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(val, 64)
		return f, err == nil
	}
	return 0, false
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func envFloat(key string, def float64) float64 {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func envInt(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
