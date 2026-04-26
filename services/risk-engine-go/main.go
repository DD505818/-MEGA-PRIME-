package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-redis/redis/v8"
)

type StreamingCorrelationMatrix struct{}

func NewStreamingCorrelationMatrix(_ int, _ float64) *StreamingCorrelationMatrix { return &StreamingCorrelationMatrix{} }

type VolatilityScaler struct{}

func NewVolatilityScaler() *VolatilityScaler { return &VolatilityScaler{} }

type RiskEngine struct {
	redis        *redis.Client
	consumer     *kafka.Consumer
	producer     *kafka.Producer
	killSwitch   atomic.Bool
	circuitBreak atomic.Bool
	correlation  *StreamingCorrelationMatrix
	volScaler    *VolatilityScaler
}

func NewRiskEngine(redisAddr, brokers string) *RiskEngine {
	addr := strings.TrimPrefix(redisAddr, "redis://")
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	c, _ := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "risk-engine",
		"auto.offset.reset": "latest",
	})
	_ = c.SubscribeTopics([]string{"signals.raw"}, nil)
	p, _ := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	return &RiskEngine{
		redis:       rdb,
		consumer:    c,
		producer:    p,
		correlation: NewStreamingCorrelationMatrix(8, 0.95),
		volScaler:   NewVolatilityScaler(),
	}
}

func (r *RiskEngine) validate(signal map[string]interface{}) (bool, string, float64) {
	if r.killSwitch.Load() {
		return false, "KILL_SWITCH_ACTIVE", 0
	}
	if r.circuitBreak.Load() {
		return false, "CIRCUIT_BREAKER_ACTIVE", 0
	}

	ctx := context.Background()
	equity, _ := r.redis.Get(ctx, "portfolio:equity").Float64()
	dailyPnL, _ := r.redis.Get(ctx, "portfolio:daily_pnl").Float64()

	if equity > 0 && dailyPnL/equity <= -0.02 {
		r.activateKillSwitch("DAILY_LOSS_LIMIT_BREACH")
		return false, "DAILY_LOSS_EXCEEDED", 0
	}

	peakEquity, _ := r.redis.Get(ctx, "portfolio:peak_equity").Float64()
	if peakEquity > 0 && equity > 0 && (peakEquity-equity)/peakEquity >= 0.10 {
		r.activateKillSwitch("MAX_DRAWDOWN_BREACH")
		return false, "MAX_DRAWDOWN_EXCEEDED", 0
	}

	price, ok := signal["limit_price"].(float64)
	if !ok || price <= 0 {
		return false, "INVALID_PRICE", 0
	}
	qty, ok := signal["quantity"].(float64)
	if !ok || qty <= 0 {
		return false, "INVALID_QUANTITY", 0
	}
	maxQty := (equity * 0.005) / (price * 0.02)
	if qty > maxQty {
		qty = maxQty
	}
	return true, "APPROVED", qty
}

func (r *RiskEngine) activateKillSwitch(reason string) {
	r.killSwitch.Store(true)
	topic := "emergency.halt"
	haltMsg, _ := json.Marshal(map[string]string{"reason": reason})
	_ = r.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          haltMsg,
	}, nil)
	_ = r.redis.Set(context.Background(), "circuit_breaker:tripped", reason, 24*time.Hour)
}

func (r *RiskEngine) run() {
	for {
		msg, err := r.consumer.ReadMessage(-1)
		if err != nil {
			continue
		}
		var signal map[string]interface{}
		_ = json.Unmarshal(msg.Value, &signal)
		valid, reason, qty := r.validate(signal)
		if !valid {
			log.Printf("rejected: %s", reason)
			continue
		}
		signal["quantity"] = qty
		payload, _ := json.Marshal(signal)
		topic := "orders.cmd"
		_ = r.producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          payload,
		}, nil)
	}
}

func main() {
	engine := NewRiskEngine(os.Getenv("REDIS_URL"), os.Getenv("KAFKA_BROKERS"))
	go engine.run()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
}
