package redis

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/piyushpatra/rate-limiter/internal/config"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
	cfg *config.Config
}

// NewClient creates a Redis client with connection pooling
// Pool is pre-warmed to avoid cold start latency on first requests
func NewClient(cfg *config.Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     cfg.RedisPoolSize,
		MinIdleConns: cfg.RedisMinIdleConns,
		
		// These timeouts are critical for fail-open behavior
		DialTimeout:  2 * time.Second,
		ReadTimeout:  cfg.RedisTimeout,
		WriteTimeout: cfg.RedisTimeout,
		
		// Pool timeout should be tight to avoid queueing requests
		PoolTimeout: 1 * time.Second,
	})

	// Verify connection on startup
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("Redis connection established successfully")

	return &Client{
		rdb: rdb,
		cfg: cfg,
	}, nil
}

// EvalLua executes a Lua script atomically
// This is the core of our rate limiting - everything happens in one round trip
func (c *Client) EvalLua(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	// Add timeout to context if not already present
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.RedisTimeout)
		defer cancel()
	}

	result, err := c.rdb.Eval(ctx, script, keys, args...).Result()
	
	// Check if error is due to Redis being unavailable or timeout
	// In production, we fail open to avoid cascading failures
	if err != nil && shouldFailOpen(err) {
		return nil, &FailOpenError{Cause: err}
	}
	
	return result, err
}

// Ping checks Redis connectivity - used by health endpoint
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close gracefully closes the Redis connection pool
func (c *Client) Close() error {
	return c.rdb.Close()
}

// FailOpenError signals that we should allow the request due to Redis issues
// This is a deliberate design choice - we prefer to be lenient vs blocking legitimate traffic
type FailOpenError struct {
	Cause error
}

func (e *FailOpenError) Error() string {
	return "redis unavailable, failing open: " + e.Cause.Error()
}

func (e *FailOpenError) Unwrap() error {
	return e.Cause
}

// shouldFailOpen determines if an error should trigger fail-open behavior
func shouldFailOpen(err error) bool {
	if err == nil {
		return false
	}
	
	// Timeout errors mean Redis is slow or unreachable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	
	// Connection errors mean Redis is down
	// TODO: might want to add circuit breaker here to avoid hammering dead Redis
	if errors.Is(err, context.Canceled) {
		return false // Don't fail open on explicit cancellation
	}
	
	// Check for network-related errors
	return isNetworkError(err)
}

func isNetworkError(err error) bool {
	// go-redis wraps network errors, so we check the error message
	// Not ideal but works reliably in practice
	errMsg := err.Error()
	return contains(errMsg, "connection refused") ||
		contains(errMsg, "connection reset") ||
		contains(errMsg, "broken pipe") ||
		contains(errMsg, "i/o timeout")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || len(s) > len(substr) && containsSlow(s, substr))
}

func containsSlow(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

