package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort   string
	RedisAddr    string
	RedisPassword string
	RedisDB      int
	
	// Connection pool settings - tuned these based on load testing
	RedisPoolSize     int
	RedisMinIdleConns int
	
	// Timeout for Redis ops - keeping it tight for fail-open behavior
	RedisTimeout time.Duration
	
	// When true, logs every request (useful for debugging but adds overhead)
	DebugLogging bool
}

// Load pulls config from environment variables with sensible defaults
func Load() *Config {
	return &Config{
		ServerPort:        getEnv("PORT", "8080"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           getEnvAsInt("REDIS_DB", 0),
		RedisPoolSize:     getEnvAsInt("REDIS_POOL_SIZE", 100),
		RedisMinIdleConns: getEnvAsInt("REDIS_MIN_IDLE_CONNS", 10),
		RedisTimeout:      getEnvAsDuration("REDIS_TIMEOUT", 2*time.Millisecond),
		DebugLogging:      getEnvAsBool("DEBUG_LOGGING", false),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if val, err := time.ParseDuration(valStr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := os.Getenv(key)
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}
	return defaultVal
}

