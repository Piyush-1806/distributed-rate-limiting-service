package api

import (
	"log"
	"net/http"
	"time"

	"github.com/piyushpatra/rate-limiter/internal/config"
)

// Logger middleware logs HTTP requests
// Only logs essentials on hot path to minimize overhead
func Logger(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Wrap response writer to capture status code
			lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			
			next.ServeHTTP(lrw, r)
			
			// Only log if debug mode is on or if there's an error
			if cfg.DebugLogging || lrw.statusCode >= 400 {
				log.Printf("[%s] %s %s - %d (%v)",
					r.Method,
					r.URL.Path,
					r.RemoteAddr,
					lrw.statusCode,
					time.Since(start),
				)
			}
		})
	}
}

// Recovery middleware recovers from panics and returns 500
// Prevents the entire server from crashing due to a single bad request
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// CORS middleware adds CORS headers
// Allowing all origins here - in production you'd want to restrict this
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

