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
	slidingWindowScript string
	slidingWindowOnce   sync.Once
)

func loadSlidingWindowScript() {
	slidingWindowOnce.Do(func() {
		// Try multiple possible paths
		paths := []string{
			"internal/redis/lua/sliding_window.lua",
			"../redis/lua/sliding_window.lua",
			"../../redis/lua/sliding_window.lua",
		}
		
		for _, path := range paths {
			if data, err := os.ReadFile(path); err == nil {
				slidingWindowScript = string(data)
				return
			}
		}
		
		// Fallback: inline the script
		slidingWindowScript = `
-- Sliding Window Log Rate Limiter
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local window_start = now - window
redis.call('ZREMRANGEBYSCORE', key, 0, window_start)
local current_count = redis.call('ZCARD', key)

local allowed = 0
local remaining = capacity - current_count

if current_count < capacity then
    local member = now .. ':' .. redis.call('INCR', key .. ':counter')
    redis.call('ZADD', key, now, member)
    allowed = 1
    remaining = remaining - 1
end

redis.call('EXPIRE', key, window + 10)
redis.call('EXPIRE', key .. ':counter', window + 10)

return {allowed, math.max(0, remaining)}
`
	})
}

// SlidingWindowLimiter implements sliding window log algorithm
// More accurate than fixed windows, prevents boundary exploits
// Uses sorted sets to track individual request timestamps
type SlidingWindowLimiter struct {
	redis *redisclient.Client
}

func NewSlidingWindowLimiter(redis *redisclient.Client) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{redis: redis}
}

// Check determines if a request should be allowed under sliding window
// capacity: max requests allowed in the window
// windowSeconds: time window in seconds
//
// Example: capacity=100, windowSeconds=60 means max 100 requests per minute
// Unlike fixed windows, this counts requests in a rolling 60-second period
func (sw *SlidingWindowLimiter) Check(ctx context.Context, key string, capacity int64, windowSeconds int64) (allowed bool, remaining int64, err error) {
	loadSlidingWindowScript() // Ensure script is loaded
	
	start := time.Now()
	defer func() {
		latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
		metrics.CheckLatency.WithLabelValues("sliding_window").Observe(latencyMs)
	}()

	if capacity <= 0 || windowSeconds <= 0 {
		return false, 0, errors.New("capacity and windowSeconds must be positive")
	}

	now := utils.NowSeconds()
	
	// Execute Lua script atomically
	// This removes old entries, counts current entries, and adds new entry in one operation
	redisStart := time.Now()
	result, err := sw.redis.EvalLua(ctx, slidingWindowScript, []string{key}, capacity, windowSeconds, now)
	redisLatency := float64(time.Since(redisStart).Microseconds()) / 1000.0
	metrics.RedisLatency.Observe(redisLatency)

	if err != nil {
		var failOpenErr *redisclient.FailOpenError
		if errors.As(err, &failOpenErr) {
			metrics.RedisErrors.Inc()
			// Fail open on Redis errors
			return true, 0, nil
		}
		return false, 0, fmt.Errorf("sliding window check failed: %w", err)
	}

	// Parse response from Lua: {allowed, remaining}
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

	if allowed {
		metrics.RequestsAllowed.WithLabelValues("sliding_window").Inc()
	} else {
		metrics.RequestsBlocked.WithLabelValues("sliding_window").Inc()
	}

	return allowed, remaining, nil
}

