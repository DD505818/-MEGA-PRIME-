package main

// redisStore is the subset of the go-redis client API used by the risk
// service. *redis.Client satisfies it in production; tests inject an
// in-memory fake so control-plane behavior (auth, kill-state persistence,
// audit) can be tested without live infrastructure.

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type redisStore interface {
	Ping(ctx context.Context) *redis.StatusCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Incr(ctx context.Context, key string) *redis.IntCmd
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
}

// controlSeqKey holds the monotonic control-plane sequence (kill/reset
// ordering) in Redis.
const controlSeqKey = "control:seq"
