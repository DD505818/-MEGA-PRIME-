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
	"github.com/go-redis/redis/v8"
)

// RiskEngine implements the 14-gate AEGIS Governor.
type RiskEngine struct {
	redis        *redis.Client
	consumer     *kafka.Consumer
	producer     *kafka.Producer
	killSwitch   atomic.Bool
	circuitBreak atomic.Bool

	mu            sync.Mutex
	seenSignalIDs map[string]struct{} // duplicate detection

	maxDailyLoss    float64
	maxDrawdown     float64
	maxPositions    int
	maxNotional     float64
	maxLeverage     float64
	maxSpreadBps    float64
	riskPerTrade    float64
	minConfidence   float64
	staleBookSecs   int64
	maxAssetExposure float64
	maxCorrelation  float64
}

func NewRiskEngine(redisAddr, brokers string) *RiskEngine {
	addr := strings.TrimPrefix(redisAddr, "redis://")
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "risk-engine",
		"auto.offset.reset": "latest",
	})
	if err != nil {
		log.Fatalf("kafka consumer: %v", err)
	}
	c.SubscribeTopics([]string{"signals.raw"}, nil)

	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		log.Fatalf("kafka producer: %v", err)
	}

	return &RiskEngine{
		redis:          rdb,
		consumer:       c,
		producer:       p,
		seenSignalIDs:  make(map[string]struct{}),
		maxDailyLoss:   envFloat("MAX_DAILY_LOSS", 0.02),
		maxDrawdown:    envFloat("MAX_DRAWDOWN", 0.10),
		maxPositions:   envInt("MAX_POSITIONS", 8),
		maxNotional:    envFloat("MAX_NOTIONAL_PER_TRADE", 50_000),
		maxLeverage:    envFloat("MAX_LEVERAGE", 2.0),
		maxSpreadBps:   envFloat("MAX_SPREAD_BPS", 20),
		riskPerTrade:   envFloat("RISK_PER_TRADE", 0.005),
		minConfidence:  envFloat("MIN_CONFIDENCE", 0.60),
		staleBookSecs:  int64(envInt("STALE_BOOK_SECONDS", 5)),
		maxAssetExposure: envFloat("MAX_ASSET_EXPOSURE_PCT", 0.25),
		maxCorrelation:  envFloat("MAX_PAIR_CORRELATION", 0.70),
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
	paperMode := os.Getenv("PAPER_MODE")
	signalMode, _ := signal["mode"].(string)
	isLiveEnv := paperMode == "" || strings.ToLower(paperMode) != "true"
	if isLiveEnv && signalMode == "paper" {
		return false, "GATE3_PAPER_SIGNAL_IN_LIVE_ENV", 0
	}

	// ── Gate 4: Broker Health ───────────────────────────────────────────────
	brokerStatus := r.redisString(ctx, "broker:status")
	if brokerStatus == "DOWN" || brokerStatus == "DEGRADED" {
		return false, "GATE4_BROKER_DOWN", 0
	}

	// ── Gate 5: Stale Market Data ───────────────────────────────────────────
	symbol, _ := signal["symbol"].(string)
	bookTSKey := fmt.Sprintf("book_ts:%s", symbol)
	bookTSStr := r.redisString(ctx, bookTSKey)
	if bookTSStr != "" {
		bookTS, _ := strconv.ParseInt(bookTSStr, 10, 64)
		ageMs := time.Now().UnixMilli() - bookTS
		if ageMs > r.staleBookSecs*1000 {
			return false, fmt.Sprintf("GATE5_STALE_BOOK_%dms", ageMs), 0
		}
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
	openPos := int(r.redisFloat(ctx, "portfolio:open_positions"))
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
	signalID, _ := signal["signal_id"].(string)
	r.mu.Lock()
	_, seen := r.seenSignalIDs[signalID]
	if seen {
		r.mu.Unlock()
		return false, "GATE11_DUPLICATE_SIGNAL_ID", 0
	}
	r.seenSignalIDs[signalID] = struct{}{}
	// Prune map if too large
	if len(r.seenSignalIDs) > 50_000 {
		r.seenSignalIDs = make(map[string]struct{})
	}
	r.mu.Unlock()

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

func (r *RiskEngine) activateKillSwitch(reason string) {
	if r.killSwitch.CompareAndSwap(false, true) {
		log.Printf("KILL SWITCH ACTIVATED: %s", reason)
		ctx := context.Background()
		r.redis.Set(ctx, "kill_switch", "1", 0)
		r.publishHalt(ctx, reason)
		// Schedule kill confirmation check after 5s
		go r.confirmKillCascade(reason)
	}
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
