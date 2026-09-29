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
	// 1B.2 position-reconciliation primitives: execution-service maintains
	// the fills ledger (lists), tracked symbols and open symbols (sets).
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SCard(ctx context.Context, key string) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	LRange(ctx context.Context, key string, start, stop int64) *redis.StringSliceCmd
	LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd
}

// controlSeqKey holds the monotonic control-plane sequence (kill/reset
// ordering) in Redis.
const controlSeqKey = "control:seq"
