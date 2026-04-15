package execution

import (
	"context"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

type IdempotencyStore struct {
	rdb *redis.Client
}

func NewIdempotencyStore() *IdempotencyStore {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "redis:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	return &IdempotencyStore{rdb: rdb}
}

func (s *IdempotencyStore) TryLock(orderID string) bool {
	ctx := context.Background()
	ok, err := s.rdb.SetNX(ctx, "order:"+orderID, "processed", 24*time.Hour).Result()
	if err != nil {
		return false
	}
	return ok
}
