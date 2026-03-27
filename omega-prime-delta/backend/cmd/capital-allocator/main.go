package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/IBM/sarama"
	_ "github.com/lib/pq"
)

var (
	mu              sync.RWMutex
	strategyWeights = map[string]float64{
		"Agent001": 0.1, "Agent002": 0.1, "Agent003": 0.1, "Agent004": 0.1,
		"Agent005": 0.1, "Agent006": 0.1, "Agent007": 0.1, "Agent008": 0.1,
		"Agent009": 0.1, "Agent010": 0.1, "Agent011": 0.1, "Agent012": 0.1,
		"Agent013": 0.1, "Agent014": 0.1, "Agent015": 0.1,
	}
)

func main() {
	// Initialize DB (same as risk engine)
	// ... (omitted for brevity)

	// Start weight consumer from meta-controller
	brokers := []string{os.Getenv("KAFKA_BROKERS")}
	consumer, err := sarama.NewConsumer(brokers, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()
	partitionConsumer, err := consumer.ConsumePartition("meta_controller.weights", 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for msg := range partitionConsumer.Messages() {
			var update map[string]float64
			if err := json.Unmarshal(msg.Value, &update); err != nil {
				log.Printf("Invalid weight update: %v", err)
				continue
			}
			mu.Lock()
			for agent, w := range update {
				if _, ok := strategyWeights[agent]; ok {
					strategyWeights[agent] = w
				}
			}
			// Normalize
			total := 0.0
			for _, w := range strategyWeights {
				total += w
			}
			if total > 0 {
				for agent := range strategyWeights {
					strategyWeights[agent] /= total
				}
			}
			mu.Unlock()
			log.Printf("Updated weights: %v", strategyWeights)
		}
	}()

	// Consume signals.generated, apply weights, produce orders.requested
	// ... (similar to previous versions)
	select {}
}
