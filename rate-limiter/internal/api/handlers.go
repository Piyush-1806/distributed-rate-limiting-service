package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/piyushpatra/rate-limiter/internal/limiter"
	redisclient "github.com/piyushpatra/rate-limiter/internal/redis"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	limiter *limiter.Limiter
	redis   *redisclient.Client
}

func NewHandler(limiter *limiter.Limiter, redis *redisclient.Client) *Handler {
	return &Handler{
		limiter: limiter,
		redis:   redis,
	}
}

// CheckRequest represents the incoming rate limit check request
type CheckRequest struct {
	Key           string  `json:"key"`
	Algorithm     string  `json:"algorithm"`
	Capacity      int64   `json:"capacity"`
	RefillRate    float64 `json:"refill_rate,omitempty"`    // for token_bucket
	WindowSeconds int64   `json:"window_seconds,omitempty"` // for sliding_window
}

// CheckResponse represents the rate limit check result
type CheckResponse struct {
	Allowed   bool  `json:"allowed"`
	Remaining int64 `json:"remaining"`
}

// HandleCheck processes rate limit check requests
// This is the hot path - keep allocations minimal
func (h *Handler) HandleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateCheckRequest(&req); err != nil {
		respondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Execute rate limit check
	result, err := h.limiter.Check(r.Context(), limiter.CheckRequest{
		Key:           req.Key,
		Algorithm:     req.Algorithm,
		Capacity:      req.Capacity,
		RefillRate:    req.RefillRate,
		WindowSeconds: req.WindowSeconds,
	})

	if err != nil {
		log.Printf("rate limit check error: %v", err)
		respondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, CheckResponse{
		Allowed:   result.Allowed,
		Remaining: result.Remaining,
	}, http.StatusOK)
}

// HandleHealth checks service health
// Returns 200 if healthy, 503 if Redis is down
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Redis connectivity
	if err := h.redis.Ping(r.Context()); err != nil {
		respondJSON(w, map[string]string{
			"status": "unhealthy",
			"error":  "redis connection failed",
		}, http.StatusServiceUnavailable)
		return
	}

	respondJSON(w, map[string]string{
		"status": "healthy",
	}, http.StatusOK)
}

// HandleMetrics exposes Prometheus metrics
func (h *Handler) HandleMetrics() http.Handler {
	return promhttp.Handler()
}

// validateCheckRequest ensures request parameters are valid
func validateCheckRequest(req *CheckRequest) error {
	if req.Key == "" {
		return &ValidationError{"key is required"}
	}

	if req.Capacity <= 0 {
		return &ValidationError{"capacity must be positive"}
	}

	switch req.Algorithm {
	case limiter.AlgorithmTokenBucket:
		if req.RefillRate <= 0 {
			return &ValidationError{"refill_rate must be positive for token_bucket"}
		}
	
	case limiter.AlgorithmSlidingWindow:
		if req.WindowSeconds <= 0 {
			return &ValidationError{"window_seconds must be positive for sliding_window"}
		}
	
	default:
		return &ValidationError{"algorithm must be 'token_bucket' or 'sliding_window'"}
	}

	return nil
}

// ValidationError represents a request validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

// respondError writes an error response
func respondError(w http.ResponseWriter, message string, status int) {
	respondJSON(w, map[string]string{
		"error": message,
	}, status)
}

