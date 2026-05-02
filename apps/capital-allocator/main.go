// Capital Allocator — Kelly-fractional position sizing with HRP overlay.
// Subscribes to signals.approved, computes optimal position size, and
// publishes sized signals to signals.sized.
package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-redis/redis/v8"
)

// KellyConfig mirrors the strategy-config.yaml kelly section.
type KellyConfig struct {
	BaseFraction   float64
	MaxFraction    float64
	DrawdownReduce float64 // fraction when drawdown >10%
	WinStreakBoost float64 // fraction when win streak >=3
}

var defaultKelly = KellyConfig{
	BaseFraction:   0.25,
	MaxFraction:    0.35,
	DrawdownReduce: 0.15,
	WinStreakBoost: 0.30,
}

type Allocator struct {
	redis    *redis.Client
	consumer *kafka.Consumer
	producer *kafka.Producer
	kelly    KellyConfig
}

func NewAllocator(redisAddr, brokers string) *Allocator {
	addr := strings.TrimPrefix(redisAddr, "redis://")
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "capital-allocator",
		"auto.offset.reset": "latest",
	})
	if err != nil {
		log.Fatalf("allocator consumer: %v", err)
	}
	c.SubscribeTopics([]string{"signals.approved"}, nil)

	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		log.Fatalf("allocator producer: %v", err)
	}

	return &Allocator{
		redis:    rdb,
		consumer: c,
		producer: p,
		kelly:    defaultKelly,
	}
}

// kellyFraction returns the effective Kelly fraction based on current state.
func (a *Allocator) kellyFraction(ctx context.Context) float64 {
	equity := a.redisFloat(ctx, "portfolio:equity")
	peak := a.redisFloat(ctx, "portfolio:peak_equity")
	if peak <= 0 {
		peak = equity
	}

	drawdown := 0.0
	if peak > 0 && equity > 0 {
		drawdown = (peak - equity) / peak
	}

	if drawdown >= 0.10 {
		return a.kelly.DrawdownReduce
	}

	winStreakStr := a.redis.Get(ctx, "portfolio:win_streak").Val()
	winStreak, _ := strconv.Atoi(winStreakStr)
	if winStreak >= 3 {
		f := a.kelly.WinStreakBoost
		if f > a.kelly.MaxFraction {
			f = a.kelly.MaxFraction
		}
		return f
	}

	return a.kelly.BaseFraction
}

// computeSize computes quantity using fractional Kelly formula.
//
//	k_full = (p*b - q) / b  where b = avg_win / avg_loss
//	k_eff = k_full * fraction
//	qty = k_eff * equity / |entry - stop|
func (a *Allocator) computeSize(
	signal map[string]interface{},
	equity, winRate, avgWin, avgLoss float64,
	kellyFrac float64,
) float64 {
	if equity <= 0 {
		return 0
	}

	price, ok := jsonFloat(signal["limit_price"])
	if !ok || price <= 0 {
		return 0
	}

	stop, hasStop := jsonFloat(signal["stop"])
	var riskDist float64
	if hasStop && stop > 0 {
		riskDist = math.Abs(price - stop)
	} else {
		riskDist = price * 0.02 // fallback 2% risk distance
	}
	if riskDist <= 0 {
		return 0
	}

	// Kelly formula
	var kFull float64
	if avgLoss > 0 && winRate > 0 {
		b := avgWin / avgLoss
		q := 1.0 - winRate
		kFull = (winRate*b - q) / b
	} else {
		kFull = 0.05 // conservative default
	}
	if kFull < 0 {
		kFull = 0
	}

	kEff := kFull * kellyFrac
	// Square-root scaling for position size growth
	scaleFactor := math.Sqrt(equity / 100_000.0)
	qty := (kEff * equity * scaleFactor) / riskDist

	// Hard caps
	maxNotional := 50_000.0
	maxQty := maxNotional / price
	if qty > maxQty {
		qty = maxQty
	}
	if qty < 0.0001 {
		return 0
	}
	return qty
}

func (a *Allocator) run() {
	log.Println("Capital allocator online")
	ctx := context.Background()

	for {
		ev := a.consumer.Poll(100)
		if ev == nil {
			continue
		}
		msg, ok := ev.(*kafka.Message)
		if !ok {
			continue
		}

		var signal map[string]interface{}
		if err := json.Unmarshal(msg.Value, &signal); err != nil {
			continue
		}

		equity := a.redisFloat(ctx, "portfolio:equity")
		if equity <= 0 {
			equity = 100_000
		}

		// Strategy performance stats (from Redis, maintained by feedback loop)
		stratID, _ := signal["strategy_id"].(string)
		winRate := a.redisFloat(ctx, "stats:win_rate:"+stratID)
		if winRate <= 0 {
			winRate = 0.50
		}
		avgWin := a.redisFloat(ctx, "stats:avg_win:"+stratID)
		if avgWin <= 0 {
			avgWin = 1.0
		}
		avgLoss := a.redisFloat(ctx, "stats:avg_loss:"+stratID)
		if avgLoss <= 0 {
			avgLoss = 1.0
		}

		kf := a.kellyFraction(ctx)
		qty := a.computeSize(signal, equity, winRate, avgWin, avgLoss, kf)
		if qty <= 0 {
			continue
		}

		signal["quantity"] = qty
		signal["kelly_fraction"] = kf
		signal["equity_snapshot"] = equity

		topic := "signals.sized"
		b, _ := json.Marshal(signal)
		_ = a.producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          b,
		}, nil)
	}
}

func (a *Allocator) redisFloat(ctx context.Context, key string) float64 {
	v, _ := a.redis.Get(ctx, key).Float64()
	return v
}

func jsonFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

func main() {
	alloc := NewAllocator(os.Getenv("REDIS_URL"), os.Getenv("KAFKA_BROKERS"))
	go alloc.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	go func() {
		log.Println("Capital allocator HTTP on :8082")
		http.ListenAndServe(":8082", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
