package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/piyushpatra/rate-limiter/internal/api"
	"github.com/piyushpatra/rate-limiter/internal/config"
	"github.com/piyushpatra/rate-limiter/internal/limiter"
	redisclient "github.com/piyushpatra/rate-limiter/internal/redis"
)

func main() {
	log.Println("Starting Rate Limiter Service...")

	// Load configuration
	cfg := config.Load()
	log.Printf("Config loaded: Redis=%s, Port=%s", cfg.RedisAddr, cfg.ServerPort)

	// Initialize Redis client
	redis, err := redisclient.NewClient(cfg)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to connect to Redis: %v", err)
		log.Println("🔓 Running in FAIL-OPEN mode - all requests will be allowed")
		log.Println("   (This demonstrates the fail-open strategy)")
		log.Println("   To run with Redis: docker run -d -p 6379:6379 redis:7-alpine")
	} else {
		defer redis.Close()
		log.Println("✅ Redis connected successfully")
	}

	// Initialize rate limiter
	rateLimiter := limiter.NewLimiter(redis)

	// Initialize HTTP handlers
	handler := api.NewHandler(rateLimiter, redis)

	// Set up router with middleware
	mux := http.NewServeMux()
	
	// API endpoints
	mux.HandleFunc("/check", handler.HandleCheck)
	mux.HandleFunc("/health", handler.HandleHealth)
	mux.Handle("/metrics", handler.HandleMetrics())

	// Apply middleware chain
	// Recovery -> CORS -> Logger -> Handler
	wrappedMux := api.Recovery(api.CORS(api.Logger(cfg)(mux)))

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      wrappedMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server listening on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Give outstanding requests 5 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}

