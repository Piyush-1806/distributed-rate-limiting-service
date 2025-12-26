package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// These metrics give us visibility into rate limiter behavior and Redis performance
// In production, we'd watch redis_latency_ms and redis_errors_total closely

var (
	// RequestsAllowed tracks successful rate limit checks by algorithm
	RequestsAllowed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_allowed_total",
			Help: "Total number of requests allowed through the rate limiter",
		},
		[]string{"algorithm"}, // token_bucket or sliding_window
	)

	// RequestsBlocked tracks rejected requests by algorithm
	RequestsBlocked = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_blocked_total",
			Help: "Total number of requests blocked by the rate limiter",
		},
		[]string{"algorithm"},
	)

	// RedisLatency measures how long Redis operations take
	// Most requests should be <1ms, alert if p99 goes over 2ms
	RedisLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "redis_latency_ms",
			Help:    "Redis operation latency in milliseconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50, 100}, // ms
		},
	)

	// RedisErrors counts Redis failures that trigger fail-open
	// Spike in this metric means Redis is having issues
	RedisErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "redis_errors_total",
			Help: "Total number of Redis errors encountered",
		},
	)

	// CheckLatency tracks end-to-end latency of rate limit checks
	CheckLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "check_latency_ms",
			Help:    "Rate limit check latency in milliseconds",
			Buckets: []float64{0.5, 1, 2, 3, 5, 10, 25, 50},
		},
		[]string{"algorithm"},
	)
)

