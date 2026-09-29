package main

// In-memory fake implementing redisStore for control-plane tests.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type fakeRedis struct {
	mu      sync.Mutex
	kv      map[string]string
	streams map[string][]map[string]interface{}
	// Failure injection for durability tests. When set, the corresponding
	// command returns the error instead of succeeding. Read under f.mu;
	// tests must use the locked setters below.
	failSetErr  error
	failGetErr  error
	failDelErr  error
	failIncrErr error
}

// Locked failure-injection setters. Command methods may be called by
// background goroutines (e.g. the boot-state retry loop), so direct field
// writes from tests would race — the race detector catches them.
func (f *fakeRedis) setFailSet(err error)  { f.mu.Lock(); defer f.mu.Unlock(); f.failSetErr = err }
func (f *fakeRedis) setFailGet(err error)  { f.mu.Lock(); defer f.mu.Unlock(); f.failGetErr = err }
func (f *fakeRedis) setFailDel(err error)  { f.mu.Lock(); defer f.mu.Unlock(); f.failDelErr = err }
func (f *fakeRedis) setFailIncr(err error) { f.mu.Lock(); defer f.mu.Unlock(); f.failIncrErr = err }

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		kv:      map[string]string{},
		streams: map[string][]map[string]interface{}{},
	}
}

func (f *fakeRedis) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("PONG")
	return cmd
}

func (f *fakeRedis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := redis.NewStatusCmd(ctx)
	if f.failSetErr != nil {
		cmd.SetErr(f.failSetErr)
		return cmd
	}
	f.kv[key] = fmt.Sprintf("%v", value)
	cmd.SetVal("OK")
	return cmd
}

func (f *fakeRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := redis.NewStringCmd(ctx)
	if f.failGetErr != nil {
		cmd.SetErr(f.failGetErr)
		return cmd
	}
	v, ok := f.kv[key]
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	cmd.SetVal(v)
	return cmd
}

func (f *fakeRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := redis.NewIntCmd(ctx)
	if f.failDelErr != nil {
		cmd.SetErr(f.failDelErr)
		return cmd
	}
	var n int64
	for _, k := range keys {
		if _, ok := f.kv[k]; ok {
			delete(f.kv, k)
			n++
		}
	}
	cmd.SetVal(n)
	return cmd
}

func (f *fakeRedis) Incr(ctx context.Context, key string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := redis.NewIntCmd(ctx)
	if f.failIncrErr != nil {
		cmd.SetErr(f.failIncrErr)
		return cmd
	}
	var n int64
	if v, ok := f.kv[key]; ok {
		fmt.Sscanf(v, "%d", &n)
	}
	n++
	f.kv[key] = fmt.Sprintf("%d", n)
	cmd.SetVal(n)
	return cmd
}

func (f *fakeRedis) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	fields := map[string]interface{}{}
	if m, ok := a.Values.(map[string]interface{}); ok {
		for k, v := range m {
			fields[k] = v
		}
	}
	f.streams[a.Stream] = append(f.streams[a.Stream], fields)
	cmd := redis.NewStringCmd(ctx)
	cmd.SetVal(fmt.Sprintf("fake-%d", len(f.streams[a.Stream])))
	return cmd
}

// streamEntries returns recorded entries for a stream (test inspection).
func (f *fakeRedis) streamEntries(stream string) []map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]interface{}{}, f.streams[stream]...)
}
