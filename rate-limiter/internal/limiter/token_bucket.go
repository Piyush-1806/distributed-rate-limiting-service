package limiter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/piyushpatra/rate-limiter/internal/metrics"
	redisclient "github.com/piyushpatra/rate-limiter/internal/redis"
	"github.com/piyushpatra/rate-limiter/internal/utils"
)

var (
	tokenBucketScript string
	tokenBucketOnce   sync.Once
)

func loadTokenBucketScript() {
	tokenBucketOnce.Do(func() {
		// Try multiple possible paths
		paths := []string{
			"internal/redis/lua/token_bucket.lua",
			"../redis/lua/token_bucket.lua",
			"../../redis/lua/token_bucket.lua",
		}
		
		for _, path := range paths {
			if data, err := os.ReadFile(path); err == nil {
				tokenBucketScript = string(data)
				return
			}
		}
		
		// Fallback: inline the script
		tokenBucketScript = `
-- Token Bucket Rate Limiter
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    last_refill = now
end

local elapsed_seconds = (now - last_refill) / 1000.0
local tokens_to_add = elapsed_seconds * refill_rate
tokens = math.min(capacity, tokens + tokens_to_add)
last_refill = now

local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
local ttl = math.ceil(capacity / refill_rate * 2)
redis.call('EXPIRE', key, ttl)

return {allowed, math.floor(tokens)}
`
	})
}

// TokenBucketLimiter implements the token bucket algorithm
// Good for allowing bursts while maintaining average rate
type TokenBucketLimiter struct {
	redis *redisclient.Client
}

func NewTokenBucketLimiter(redis *redisclient.Client) *TokenBucketLimiter {
	return &TokenBucketLimiter{redis: redis}
}

// Check determines if a request should be allowed under token bucket
// capacity: max tokens in bucket (allows bursts up to this size)
// refillRate: tokens added per second (average rate limit)
func (tb *TokenBucketLimiter) Check(ctx context.Context, key string, capacity int64, refillRate float64) (allowed bool, remaining int64, err error) {
	loadTokenBucketScript() // Ensure script is loaded
	
	start := time.Now()
	defer func() {
		// Track latency for this algorithm
		latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
		metrics.CheckLatency.WithLabelValues("token_bucket").Observe(latencyMs)
	}()

	if capacity <= 0 || refillRate <= 0 {
		return false, 0, errors.New("capacity and refillRate must be positive")
	}

	now := utils.NowMillis()
	
	// Execute Lua script atomically
	redisStart := time.Now()
	result, err := tb.redis.EvalLua(ctx, tokenBucketScript, []string{key}, capacity, refillRate, now)
	redisLatency := float64(time.Since(redisStart).Microseconds()) / 1000.0
	metrics.RedisLatency.Observe(redisLatency)

	if err != nil {
		// Check if this is a fail-open error
		var failOpenErr *redisclient.FailOpenError
		if errors.As(err, &failOpenErr) {
			metrics.RedisErrors.Inc()
			// Fail open: allow request when Redis is unavailable
			// This prevents rate limiter from becoming a single point of failure
			return true, 0, nil
		}
		return false, 0, fmt.Errorf("token bucket check failed: %w", err)
	}

	// Parse Lua response: {allowed, remaining}
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) != 2 {
		return false, 0, errors.New("unexpected response format from Lua script")
	}

	allowedInt, ok1 := resultSlice[0].(int64)
	remainingInt, ok2 := resultSlice[1].(int64)
	if !ok1 || !ok2 {
		return false, 0, errors.New("failed to parse Lua script response")
	}

	allowed = allowedInt == 1
	remaining = remainingInt

	// Update metrics
	if allowed {
		metrics.RequestsAllowed.WithLabelValues("token_bucket").Inc()
	} else {
		metrics.RequestsBlocked.WithLabelValues("token_bucket").Inc()
	}

	return allowed, remaining, nil
}

