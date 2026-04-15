package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omega-prime/omega-prime-delta/backend/internal/execution"
	"github.com/omega-prime/omega-prime-delta/backend/internal/models"
	"github.com/segmentio/kafka-go"
)

func main() {
	kafkaBrokers := getEnv("KAFKA_BROKERS", "kafka:9092")

	approvedReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaBrokers},
		Topic:   "orders.approved",
		GroupID: "execution-engine",
	})
	defer approvedReader.Close()

	executedWriter := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBrokers),
		Topic:    "orders.executed",
		Balancer: &kafka.LeastBytes{},
	}
	defer executedWriter.Close()

	rejectedWriter := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBrokers),
		Topic:    "orders.rejected",
		Balancer: &kafka.LeastBytes{},
	}
	defer rejectedWriter.Close()

	riskGate := execution.NewRiskGate(getEnv("RISK_ENGINE_URL", "http://risk-engine:3002"))
	idempotency := execution.NewIdempotencyStore()
	router := execution.NewRouter()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("Execution engine started - listening on orders.approved")

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down execution engine")
			return
		default:
			msg, err := approvedReader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Read error: %v", err)
				continue
			}

			var order models.Order
			if err := json.Unmarshal(msg.Value, &order); err != nil {
				log.Printf("Unmarshal error: %v", err)
				continue
			}

			orderID := order.EffectiveID()
			if orderID == "" {
				log.Printf("Order missing id: %+v", order)
				rejectOrder(rejectedWriter, order, "missing_order_id")
				continue
			}

			if !idempotency.TryLock(orderID) {
				log.Printf("Duplicate order rejected: %s", orderID)
				rejectOrder(rejectedWriter, order, "duplicate_idempotency")
				continue
			}

			if err := riskGate.Validate(order); err != nil {
				log.Printf("Risk rejection: %v", err)
				rejectOrder(rejectedWriter, order, err.Error())
				continue
			}

			fill, err := router.Execute(order)
			if err != nil {
				log.Printf("Execution error: %v", err)
				rejectOrder(rejectedWriter, order, err.Error())
				continue
			}

			fillBytes, _ := json.Marshal(fill)
			if err := executedWriter.WriteMessages(ctx, kafka.Message{Value: fillBytes}); err != nil {
				log.Printf("Failed to emit fill for order %s: %v", orderID, err)
				if rejectErr := rejectOrder(rejectedWriter, order, "execution_publish_failed"); rejectErr != nil {
					log.Printf("Failed to emit execution publish rejection for order %s: %v", orderID, rejectErr)
				}
				continue
			}

			log.Printf("Order executed: %s -> fill %s", fill.OrderID, fill.FillID)
		}
	}
}

func rejectOrder(writer *kafka.Writer, order models.Order, reason string) error {
	rejection := models.OrderRejection{
		OrderID:   order.EffectiveID(),
		Reason:    reason,
		Timestamp: time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(rejection)
	return writer.WriteMessages(context.Background(), kafka.Message{Value: data})
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
