package main

import (
    "context"
    "encoding/json"
    "strings"
    "sync/atomic"

    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
    "github.com/go-redis/redis/v8"
)

type RiskEngine struct {
    redis        *redis.Client
    consumer     *kafka.Consumer
    producer     *kafka.Producer
    killSwitch   atomic.Bool
    circuitBreak atomic.Bool
}

func NewRiskEngine(redisAddr, brokers string) *RiskEngine {
    addr := strings.TrimPrefix(redisAddr, "redis://")
    rdb := redis.NewClient(&redis.Options{Addr: addr})
    c, _ := kafka.NewConsumer(&kafka.ConfigMap{"bootstrap.servers": brokers, "group.id": "risk-engine", "auto.offset.reset": "latest"})
    c.SubscribeTopics([]string{"signals.raw"}, nil)
    p, _ := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
    return &RiskEngine{redis: rdb, consumer: c, producer: p}
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
    price := signal["limit_price"].(float64)
    if price <= 0 {
        return false, "INVALID_PRICE", 0
    }
    qty := signal["quantity"].(float64)
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
    _ = r.producer.Produce(&kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Value: haltMsg}, nil)
}

func (r *RiskEngine) run() {}
