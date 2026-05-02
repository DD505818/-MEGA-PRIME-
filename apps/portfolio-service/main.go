// Portfolio Service — real-time P&L tracking, position management, and equity curve.
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
	"github.com/jackc/pgx/v5/pgxpool"
)

type Position struct {
	Symbol    string  `json:"symbol"`
	Qty       float64 `json:"quantity"`
	AvgCost   float64 `json:"avg_cost"`
	MarketVal float64 `json:"market_value"`
	UnrealPnL float64 `json:"unrealized_pnl"`
	RealPnL   float64 `json:"realized_pnl"`
}

type Portfolio struct {
	Equity     float64              `json:"equity"`
	PeakEquity float64              `json:"peak_equity"`
	DailyPnL   float64              `json:"daily_pnl"`
	TotalPnL   float64              `json:"total_pnl"`
	Drawdown   float64              `json:"drawdown_pct"`
	Positions  map[string]*Position `json:"positions"`
	UpdatedAt  int64                `json:"updated_at_ms"`
}

type PortfolioService struct {
	redis    *redis.Client
	db       *pgxpool.Pool
	consumer *kafka.Consumer

	mu        sync.RWMutex
	portfolio Portfolio

	baseEquity float64
}

func NewPortfolioService(redisAddr, brokers, pgDSN string) *PortfolioService {
	addr := strings.TrimPrefix(redisAddr, "redis://")
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "portfolio-service",
		"auto.offset.reset": "latest",
	})
	if err != nil {
		log.Fatalf("portfolio kafka: %v", err)
	}
	c.SubscribeTopics([]string{"orders.fills", "market.prices"}, nil)

	var db *pgxpool.Pool
	if pgDSN != "" {
		db, err = pgxpool.New(context.Background(), pgDSN)
		if err != nil {
			log.Printf("portfolio postgres: %v (continuing without DB)", err)
		}
	}

	baseEquity := 100_000.0

	return &PortfolioService{
		redis:      rdb,
		db:         db,
		consumer:   c,
		baseEquity: baseEquity,
		portfolio: Portfolio{
			Equity:     baseEquity,
			PeakEquity: baseEquity,
			Positions:  make(map[string]*Position),
		},
	}
}

func (ps *PortfolioService) run() {
	log.Println("Portfolio service online")
	ctx := context.Background()

	for {
		ev := ps.consumer.Poll(100)
		if ev == nil {
			continue
		}
		msg, ok := ev.(*kafka.Message)
		if !ok {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			continue
		}

		topic := *msg.TopicPartition.Topic
		switch topic {
		case "orders.fills":
			ps.processFill(ctx, payload)
		case "market.prices":
			ps.updateMarketPrices(payload)
		}

		ps.publishState(ctx)
	}
}

func (ps *PortfolioService) processFill(ctx context.Context, fill map[string]interface{}) {
	symbol, _ := fill["symbol"].(string)
	side, _ := fill["side"].(string)
	qty, _ := jsonF64(fill["filled_qty"])
	fillPrice, _ := jsonF64(fill["avg_fill_price"])
	if symbol == "" || qty <= 0 || fillPrice <= 0 {
		return
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	pos, ok := ps.portfolio.Positions[symbol]
	if !ok {
		pos = &Position{Symbol: symbol}
		ps.portfolio.Positions[symbol] = pos
	}

	if side == "BUY" {
		totalCost := pos.AvgCost*pos.Qty + fillPrice*qty
		pos.Qty += qty
		if pos.Qty > 0 {
			pos.AvgCost = totalCost / pos.Qty
		}
	} else if side == "SELL" {
		realPnL := (fillPrice - pos.AvgCost) * math.Min(qty, pos.Qty)
		pos.RealPnL += realPnL
		pos.Qty -= qty
		if pos.Qty <= 0 {
			pos.Qty = 0
			pos.AvgCost = 0
		}
		ps.portfolio.DailyPnL += realPnL
		ps.portfolio.TotalPnL += realPnL
	}

	// Persist to DB
	if ps.db != nil {
		ps.persistFill(ctx, fill)
	}

	// Update equity
	ps.recomputeEquity()
}

func (ps *PortfolioService) updateMarketPrices(prices map[string]interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for sym, v := range prices {
		price, ok := jsonF64(v)
		if !ok || price <= 0 {
			continue
		}
		if pos, exists := ps.portfolio.Positions[sym]; exists && pos.Qty != 0 {
			pos.MarketVal = price * pos.Qty
			pos.UnrealPnL = (price - pos.AvgCost) * pos.Qty
		}
	}
	ps.recomputeEquity()
}

func (ps *PortfolioService) recomputeEquity() {
	totalUnreal := 0.0
	for _, pos := range ps.portfolio.Positions {
		totalUnreal += pos.UnrealPnL
	}
	ps.portfolio.Equity = ps.baseEquity + ps.portfolio.TotalPnL + totalUnreal
	if ps.portfolio.Equity > ps.portfolio.PeakEquity {
		ps.portfolio.PeakEquity = ps.portfolio.Equity
	}
	if ps.portfolio.PeakEquity > 0 {
		ps.portfolio.Drawdown = (ps.portfolio.PeakEquity - ps.portfolio.Equity) / ps.portfolio.PeakEquity
	}
	ps.portfolio.UpdatedAt = time.Now().UnixMilli()
}

func (ps *PortfolioService) publishState(ctx context.Context) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	ps.redis.Set(ctx, "portfolio:equity", ps.portfolio.Equity, 0)
	ps.redis.Set(ctx, "portfolio:peak_equity", ps.portfolio.PeakEquity, 0)
	ps.redis.Set(ctx, "portfolio:daily_pnl", ps.portfolio.DailyPnL, 0)
	ps.redis.Set(ctx, "portfolio:open_positions", len(ps.portfolio.Positions), 0)
	ps.redis.Set(ctx, "portfolio:drawdown", ps.portfolio.Drawdown, 0)
}

func (ps *PortfolioService) persistFill(ctx context.Context, fill map[string]interface{}) {
	if ps.db == nil {
		return
	}
	_, _ = ps.db.Exec(ctx,
		`INSERT INTO fills (order_id, signal_id, strategy_id, symbol, side, quantity, fill_price, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		 ON CONFLICT DO NOTHING`,
		fill["order_id"], fill["signal_id"], fill["strategy_id"],
		fill["symbol"], fill["side"], fill["filled_qty"], fill["avg_fill_price"],
	)
}

func (ps *PortfolioService) portfolioHandler(w http.ResponseWriter, r *http.Request) {
	ps.mu.RLock()
	p := ps.portfolio
	ps.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (ps *PortfolioService) positionsHandler(w http.ResponseWriter, r *http.Request) {
	ps.mu.RLock()
	positions := make([]*Position, 0, len(ps.portfolio.Positions))
	for _, p := range ps.portfolio.Positions {
		positions = append(positions, p)
	}
	ps.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(positions)
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

func main() {
	svc := NewPortfolioService(
		os.Getenv("REDIS_URL"),
		os.Getenv("KAFKA_BROKERS"),
		os.Getenv("POSTGRES_DSN"),
	)
	go svc.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/portfolio", svc.portfolioHandler)
	mux.HandleFunc("/positions", svc.positionsHandler)

	go func() {
		log.Println("Portfolio service HTTP on :8083")
		http.ListenAndServe(":8083", mux)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
}
