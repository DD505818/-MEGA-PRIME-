package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/IBM/sarama"
)

func estimateSlippage(order map[string]interface{}) float64 {
	url := os.Getenv("IMPACT_MODEL_URL") + "/predict"
	data, _ := json.Marshal(order)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Impact model error: %v", err)
		return 0.001 // default 10 bps
	}
	defer resp.Body.Close()

	var result map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0.001
	}
	if slippage, ok := result["slippage"]; ok {
		return slippage
	}
	return 0.001
}

func main() {
	brokers := []string{os.Getenv("KAFKA_BROKERS")}
	consumer, err := sarama.NewConsumer(brokers, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	partitionConsumer, err := consumer.ConsumePartition("orders.requested", 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatal(err)
	}
	defer partitionConsumer.Close()

	for msg := range partitionConsumer.Messages() {
		if msg == nil {
			continue
		}
		var order map[string]interface{}
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			log.Printf("Invalid order payload: %v", err)
			continue
		}

		slippage := estimateSlippage(order)
		if qty, ok := order["qty"].(float64); ok && slippage < 0.0005 {
			order["qty"] = qty * 1.2
		}
		log.Printf("Prepared order for execution: symbol=%v qty=%v slippage=%.6f", order["symbol"], order["qty"], slippage)
		// TODO: route to concrete broker adapters and publish fills
	}
}
