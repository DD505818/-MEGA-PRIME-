package main

import (
	"log"
	"os"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-redis/redis/v8"
)

func newRedisClient(rawURL string) *redis.Client {
	if rawURL == "" {
		rawURL = "redis://redis:6379"
	}
	if strings.Contains(rawURL, "://") {
		opts, err := redis.ParseURL(rawURL)
		if err != nil {
			log.Fatalf("invalid REDIS_URL: %v", err)
		}
		return redis.NewClient(opts)
	}
	return redis.NewClient(&redis.Options{Addr: rawURL})
}

func kafkaTransport(config kafka.ConfigMap) *kafka.ConfigMap {
	copyEnv := func(envKey, configKey string) {
		if value := os.Getenv(envKey); value != "" {
			config[configKey] = value
		}
	}
	copyEnv("KAFKA_SECURITY_PROTOCOL", "security.protocol")
	copyEnv("KAFKA_SASL_MECHANISM", "sasl.mechanism")
	copyEnv("KAFKA_SASL_USERNAME", "sasl.username")
	copyEnv("KAFKA_SASL_PASSWORD", "sasl.password")
	copyEnv("KAFKA_SSL_CA_LOCATION", "ssl.ca.location")
	return &config
}
