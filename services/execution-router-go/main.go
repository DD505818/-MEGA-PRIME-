package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-redis/redis/v8"
	"github.com/shopspring/decimal"
)

type OrderState string

const (
	NEW       OrderState = "NEW"
	OPEN      OrderState = "OPEN"
	PARTIAL   OrderState = "PARTIAL"
	FILLED    OrderState = "FILLED"
	CANCELLED OrderState = "CANCELLED"
)

type Order struct {
	ID     string
	State  OrderState
	Symbol string
	Side   string
	Qty    decimal.Decimal
	Price  decimal.Decimal
}

type ExecutionEngine struct {
	redis    *redis.Client
	consumer *kafka.Consumer
	producer *kafka.Producer
	orders   map[string]*Order
	mu       sync.Mutex
}

func NewExecutionEngine(redisAddr, brokers string) *ExecutionEngine {
	addr := strings.TrimPrefix(redisAddr, "redis://")
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	c, _ := kafka.NewConsumer(&kafka.ConfigMap{"bootstrap.servers": brokers, "group.id": "execution-engine", "auto.offset.reset": "latest"})
	_ = c.SubscribeTopics([]string{"orders.cmd"}, nil)
	p, _ := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	return &ExecutionEngine{redis: rdb, consumer: c, producer: p, orders: make(map[string]*Order)}
}

func (e *ExecutionEngine) execute(order *Order) {
	e.mu.Lock()
	defer e.mu.Unlock()
	order.State = OPEN
	e.orders[order.ID] = order
	log.Printf("Order %s opened", order.ID)
}

func (e *ExecutionEngine) cancelAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, o := range e.orders {
		if o.State != FILLED && o.State != CANCELLED {
			o.State = CANCELLED
			log.Printf("Cancelled %s", id)
		}
	}
}

func (e *ExecutionEngine) run() {
	haltConsumer, _ := kafka.NewConsumer(&kafka.ConfigMap{"bootstrap.servers": os.Getenv("KAFKA_BROKERS"), "group.id": "exec-halt", "auto.offset.reset": "latest"})
	_ = haltConsumer.SubscribeTopics([]string{"emergency.halt"}, nil)
	go func() {
		for {
			msg, err := haltConsumer.ReadMessage(-1)
			if err != nil {
				continue
			}
			var halt map[string]string
			_ = json.Unmarshal(msg.Value, &halt)
			log.Printf("Emergency halt: %s", halt["reason"])
			e.cancelAll()
		}
	}()

	for {
		msg, err := e.consumer.ReadMessage(-1)
		if err != nil {
			continue
		}
		var orderData map[string]interface{}
		_ = json.Unmarshal(msg.Value, &orderData)
		order := &Order{
			ID:     orderData["order_id"].(string),
			Symbol: orderData["symbol"].(string),
			Side:   orderData["side"].(string),
			Qty:    decimal.NewFromFloat(orderData["quantity"].(float64)),
			Price:  decimal.NewFromFloat(orderData["limit_price"].(float64)),
			State:  NEW,
		}
		e.execute(order)
	}
}

func main() {
	engine := NewExecutionEngine(os.Getenv("REDIS_URL"), os.Getenv("KAFKA_BROKERS"))
	go engine.run()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
}
