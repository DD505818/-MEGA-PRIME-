// Fusion Engine — CAFÉ-RC (Confidence-Adjusted Fusion Engine with Regime Context).
//
// Subscribes to signals.raw, aggregates signals from all agents within a
// configurable window, applies confidence weighting and regime-context
// filtering, and publishes consensus signals to signals.fused.
//
// UCB1 bandit selects top-3 agents per regime for routing priority.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// ── Regime types ─────────────────────────────────────────────────────────────

type Regime string

const (
	RegimeTrending      Regime = "TRENDING"
	RegimeMeanReverting Regime = "MEAN_REVERTING"
	RegimeHighVol       Regime = "HIGH_VOL"
	RegimeCrisis        Regime = "CRISIS"
	RegimeNeutral       Regime = "NEUTRAL"
)

// AgentStats tracks UCB1 bandit statistics per agent.
type AgentStats struct {
	Name    string
	NTries  int
	XBar    float64 // mean reward (Sharpe contribution)
}

// UCB1 score for exploration/exploitation balance.
func (a *AgentStats) UCB1Score(totalN int) float64 {
	if a.NTries == 0 {
		return math.Inf(1) // unsampled agents get infinite score
	}
	return a.XBar + 1.414*math.Sqrt(math.Log(float64(totalN))/float64(a.NTries))
}

// ── Fusion Engine ─────────────────────────────────────────────────────────────

type FusionEngine struct {
	redis    *redis.Client
	consumer *kafka.Consumer
	producer *kafka.Producer

	mu         sync.Mutex
	window     []map[string]interface{}
	windowSize time.Duration
	lastFlush  time.Time
	agentStats map[string]*AgentStats
}

func NewFusionEngine(redisAddr, brokers string) *FusionEngine {
	addr := strings.TrimPrefix(redisAddr, "redis://")
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "fusion-engine",
		"auto.offset.reset": "latest",
	})
	if err != nil {
		log.Fatalf("fusion kafka consumer: %v", err)
	}
	c.SubscribeTopics([]string{"signals.raw"}, nil)

	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		log.Fatalf("fusion kafka producer: %v", err)
	}

	return &FusionEngine{
		redis:      rdb,
		consumer:   c,
		producer:   p,
		windowSize: 5 * time.Second,
		lastFlush:  time.Now(),
		agentStats: make(map[string]*AgentStats),
	}
}

func (fe *FusionEngine) run() {
	log.Println("CAFÉ-RC Fusion Engine online")
	ticker := time.NewTicker(fe.windowSize)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fe.flush()
		default:
			ev := fe.consumer.Poll(50)
			if ev == nil {
				continue
			}
			msg, ok := ev.(*kafka.Message)
			if !ok {
				continue
			}
			var signal map[string]interface{}
			if err := json.Unmarshal(msg.Value, &signal); err == nil {
				fe.mu.Lock()
				fe.window = append(fe.window, signal)
				fe.mu.Unlock()
			}
		}
	}
}

// flush aggregates the current window into consensus signals.
func (fe *FusionEngine) flush() {
	fe.mu.Lock()
	batch := fe.window
	fe.window = nil
	fe.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	ctx := context.Background()
	regime := fe.detectRegime(ctx)

	// Filter by regime compatibility
	eligible := fe.filterByRegime(batch, regime)
	if len(eligible) == 0 {
		return
	}

	// Group by symbol+side
	groups := make(map[string][]map[string]interface{})
	for _, s := range eligible {
		sym, _ := s["symbol"].(string)
		side, _ := s["side"].(string)
		key := sym + ":" + side
		groups[key] = append(groups[key], s)
	}

	for _, group := range groups {
		if len(group) < 2 {
			continue // require at least 2 agreeing agents
		}
		fused := fe.fuseGroup(group, regime)
		if fused != nil {
			fe.publish("signals.fused", fused)
			fe.updateUCB1(group)
		}
	}
}

// fuseGroup merges a group of agreeing signals via confidence-weighted averaging.
func (fe *FusionEngine) fuseGroup(
	signals []map[string]interface{},
	regime Regime,
) map[string]interface{} {
	totalWeight := 0.0
	weightedPrice := 0.0
	weightedStop := 0.0
	weightedTarget := 0.0
	maxConf := 0.0

	for _, s := range signals {
		conf, _ := jsonF64(s["confidence"])
		price, _ := jsonF64(s["limit_price"])
		stop, _ := jsonF64(s["stop"])
		target, _ := jsonF64(s["target"])
		if price <= 0 || conf <= 0 {
			continue
		}
		totalWeight += conf
		weightedPrice += conf * price
		weightedStop += conf * stop
		weightedTarget += conf * target
		if conf > maxConf {
			maxConf = conf
		}
	}

	if totalWeight <= 0 {
		return nil
	}

	// Regime-adjusted confidence multiplier
	regimeBoost := regimeConfidenceBoost(regime)
	fusedConf := math.Min(0.95, (totalWeight/float64(len(signals)))*regimeBoost)

	if fusedConf < 0.65 {
		return nil
	}

	sym, _ := signals[0]["symbol"].(string)
	side, _ := signals[0]["side"].(string)

	contributors := make([]string, 0, len(signals))
	for _, s := range signals {
		if id, ok := s["strategy_id"].(string); ok {
			contributors = append(contributors, id)
		}
	}

	return map[string]interface{}{
		"signal_id":    uuid.NewString(),
		"strategy_id":  "CAFE_RC_FUSION",
		"symbol":       sym,
		"side":         side,
		"quantity":     signals[0]["quantity"],
		"limit_price":  weightedPrice / totalWeight,
		"stop":         weightedStop / totalWeight,
		"target":       weightedTarget / totalWeight,
		"confidence":   fusedConf,
		"timestamp":    time.Now().UnixMilli(),
		"mode":         signals[0]["mode"],
		"regime":       string(regime),
		"contributors": contributors,
		"n_agreeing":   len(signals),
		"reason": fmt.Sprintf(
			"CAFÉ-RC fusion: %d agents agree %s %s; regime=%s conf=%.2f",
			len(signals), side, sym, regime, fusedConf,
		),
	}
}

func (fe *FusionEngine) detectRegime(ctx context.Context) Regime {
	r := fe.redis.Get(ctx, "market:regime").Val()
	switch Regime(r) {
	case RegimeTrending, RegimeMeanReverting, RegimeHighVol, RegimeCrisis:
		return Regime(r)
	}
	return RegimeNeutral
}

// filterByRegime removes signals from agents disabled in the current regime.
func (fe *FusionEngine) filterByRegime(
	signals []map[string]interface{},
	regime Regime,
) []map[string]interface{} {
	disabled := regimeDisabled(regime)
	out := signals[:0]
	for _, s := range signals {
		id, _ := s["strategy_id"].(string)
		if _, skip := disabled[strings.ToUpper(id)]; !skip {
			out = append(out, s)
		}
	}
	return out
}

func (fe *FusionEngine) updateUCB1(signals []map[string]interface{}) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	for _, s := range signals {
		id, _ := s["strategy_id"].(string)
		conf, _ := jsonF64(s["confidence"])
		stat, ok := fe.agentStats[id]
		if !ok {
			stat = &AgentStats{Name: id}
			fe.agentStats[id] = stat
		}
		n := stat.NTries + 1
		stat.XBar = (stat.XBar*float64(stat.NTries) + conf) / float64(n)
		stat.NTries = n
	}
}

func (fe *FusionEngine) publish(topic string, signal map[string]interface{}) {
	b, _ := json.Marshal(signal)
	_ = fe.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          b,
	}, nil)
}

// ── Regime helpers ────────────────────────────────────────────────────────────

func regimeDisabled(r Regime) map[string]struct{} {
	switch r {
	case RegimeTrending:
		return map[string]struct{}{"ARB": {}, "OPT": {}}
	case RegimeMeanReverting:
		return map[string]struct{}{"SURGE": {}, "NEXUS": {}}
	case RegimeHighVol:
		return map[string]struct{}{"SURGE": {}, "NEXUS": {}, "BOXTHEORY": {}, "GAP": {}}
	case RegimeCrisis:
		// Close everything except NEXUS hedging
		return map[string]struct{}{
			"BOXTHEORY": {}, "SURGE": {}, "ARB": {}, "GAP": {},
			"REV": {}, "SENTI": {}, "TWIN": {}, "MAKER": {},
			"HARVEST": {}, "GOLD": {}, "OPT": {},
		}
	}
	return map[string]struct{}{}
}

func regimeConfidenceBoost(r Regime) float64 {
	switch r {
	case RegimeTrending:
		return 1.05
	case RegimeMeanReverting:
		return 1.03
	case RegimeHighVol:
		return 0.92
	case RegimeCrisis:
		return 0.80
	}
	return 1.0
}

func jsonF64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

// ── HTTP ─────────────────────────────────────────────────────────────────────

func (fe *FusionEngine) statsHandler(w http.ResponseWriter, r *http.Request) {
	fe.mu.Lock()
	stats := make([]*AgentStats, 0, len(fe.agentStats))
	for _, s := range fe.agentStats {
		stats = append(stats, s)
	}
	fe.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func main() {
	fe := NewFusionEngine(os.Getenv("REDIS_URL"), os.Getenv("KAFKA_BROKERS"))
	go fe.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/stats", fe.statsHandler)

	go func() {
		log.Println("Fusion Engine HTTP on :8085")
		http.ListenAndServe(":8085", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
