// Package ratelimit provides a Redis-backed token bucket rate limiter
// with tiered limits for anonymous, authenticated, and trusted users.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// TierConfig defines rate limit parameters for a user tier.
type TierConfig struct {
	Limit     int
	WindowSec int
}

// TierLimits defines global request rate limits per user tier.
var TierLimits = map[string]TierConfig{
	"anonymous":     {Limit: 10, WindowSec: 60},
	"authenticated": {Limit: 100, WindowSec: 60},
}

// ActionLimits defines per-action rate limits (called from service handlers).
var ActionLimits = map[string]TierConfig{
	"post":      {Limit: 5, WindowSec: 3600},
	"comment":   {Limit: 30, WindowSec: 3600},
	"vote":      {Limit: 60, WindowSec: 60},
	"community": {Limit: 1, WindowSec: 86400},
}

// Result holds the outcome of a rate limit check.
type Result struct {
	Allowed    bool
	Remaining  int
	Limit      int
	RetryAfter time.Duration
}

// Limiter performs rate limit checks using Redis as the backing store.
type Limiter struct {
	rdb *goredis.Client
}

// New creates a new Limiter backed by the given Redis client.
func New(rdb *goredis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

// tokenBucketScript is a Lua script that atomically checks and decrements
// a token bucket in Redis. Returns {allowed (0/1), remaining, ttl_ms}.
var tokenBucketScript = goredis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = tonumber(redis.call("GET", key) or "0")

if current >= limit then
    local ttl = redis.call("PTTL", key)
    if ttl < 0 then ttl = window * 1000 end
    return {0, 0, ttl}
end

local new_val = redis.call("INCR", key)
if new_val == 1 then
    redis.call("PEXPIRE", key, window * 1000)
end

local remaining = limit - new_val
if remaining < 0 then remaining = 0 end

local ttl = redis.call("PTTL", key)
if ttl < 0 then ttl = window * 1000 end

return {1, remaining, ttl}
`)

// Check performs a rate limit check for the given key with the specified
// limit and window. Returns the result indicating whether the request is allowed.
func (l *Limiter) Check(ctx context.Context, key string, limit int, windowSeconds int) (*Result, error) {
	vals, err := tokenBucketScript.Run(ctx, l.rdb, []string{key}, limit, windowSeconds).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("rate limit check: %w", err)
	}

	if len(vals) < 3 {
		return nil, fmt.Errorf("rate limit script returned unexpected result")
	}

	allowed := vals[0] == 1
	remaining := int(vals[1])
	ttlMs := vals[2]

	result := &Result{
		Allowed:   allowed,
		Remaining: remaining,
		Limit:     limit,
	}

	if !allowed {
		result.RetryAfter = time.Duration(ttlMs) * time.Millisecond
	}

	return result, nil
}
