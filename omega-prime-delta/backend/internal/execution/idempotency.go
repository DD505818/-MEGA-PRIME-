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

func NewIdempotencyStore(redisAddr string) *IdempotencyStore {
	if redisAddr == "" {
		redisAddr = os.Getenv("REDIS_ADDR")
	}
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
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
