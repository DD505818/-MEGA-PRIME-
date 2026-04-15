package execution

import (
    "context"
    "time"

    "github.com/go-redis/redis/v8"
)

type IdempotencyStore struct {
    rdb *redis.Client
}

func NewIdempotencyStore() *IdempotencyStore {
    rdb := redis.NewClient(&redis.Options{
        Addr: "redis:6379",
    })
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
