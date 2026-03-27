package kafka

import (
	"testing"

	"github.com/IBM/sarama"
)

func TestProducerConfigIdempotenceRequirements(t *testing.T) {
	config := newProducerConfig()

	if !config.Producer.Idempotent {
		t.Fatalf("Producer.Idempotent = false, want true")
	}
	if config.Net.MaxOpenRequests != 1 {
		t.Fatalf("Net.MaxOpenRequests = %d, want 1", config.Net.MaxOpenRequests)
	}
	if config.Producer.RequiredAcks != sarama.WaitForAll {
		t.Fatalf("Producer.RequiredAcks = %v, want %v", config.Producer.RequiredAcks, sarama.WaitForAll)
	}
	if !config.Producer.Return.Successes {
		t.Fatalf("Producer.Return.Successes = false, want true")
	}
}
