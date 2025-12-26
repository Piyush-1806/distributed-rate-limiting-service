package utils

import "time"

// NowMillis returns current Unix timestamp in milliseconds
// Using milliseconds instead of seconds for better precision in token bucket refills
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// NowSeconds returns current Unix timestamp in seconds
// Used for sliding window where second-level precision is enough
func NowSeconds() int64 {
	return time.Now().Unix()
}

