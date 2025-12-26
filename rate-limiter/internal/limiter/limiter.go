package limiter

import (
	"context"
	"errors"
	"fmt"

	redisclient "github.com/piyushpatra/rate-limiter/internal/redis"
)

// Algorithm types supported by the rate limiter
const (
	AlgorithmTokenBucket   = "token_bucket"
	AlgorithmSlidingWindow = "sliding_window"
)

// Limiter provides a unified interface for different rate limiting algorithms
type Limiter struct {
	tokenBucket   *TokenBucketLimiter
	slidingWindow *SlidingWindowLimiter
}

// NewLimiter creates a new rate limiter with both algorithms
func NewLimiter(redis *redisclient.Client) *Limiter {
	return &Limiter{
		tokenBucket:   NewTokenBucketLimiter(redis),
		slidingWindow: NewSlidingWindowLimiter(redis),
	}
}

// CheckRequest evaluates a rate limit check based on the specified algorithm
type CheckRequest struct {
	Key           string
	Algorithm     string
	Capacity      int64
	RefillRate    float64 // only for token bucket
	WindowSeconds int64   // only for sliding window
}

type CheckResponse struct {
	Allowed   bool
	Remaining int64
}

// Check routes the request to the appropriate algorithm
// This is the main entry point for rate limiting decisions
func (l *Limiter) Check(ctx context.Context, req CheckRequest) (*CheckResponse, error) {
	if req.Key == "" {
		return nil, errors.New("key cannot be empty")
	}

	var allowed bool
	var remaining int64
	var err error

	switch req.Algorithm {
	case AlgorithmTokenBucket:
		allowed, remaining, err = l.tokenBucket.Check(ctx, req.Key, req.Capacity, req.RefillRate)
	
	case AlgorithmSlidingWindow:
		allowed, remaining, err = l.slidingWindow.Check(ctx, req.Key, req.Capacity, req.WindowSeconds)
	
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s (supported: %s, %s)", 
			req.Algorithm, AlgorithmTokenBucket, AlgorithmSlidingWindow)
	}

	if err != nil {
		return nil, err
	}

	return &CheckResponse{
		Allowed:   allowed,
		Remaining: remaining,
	}, nil
}

