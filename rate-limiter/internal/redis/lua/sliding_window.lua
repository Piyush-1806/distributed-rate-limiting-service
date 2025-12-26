-- Sliding Window Log Rate Limiter
-- KEYS[1]: rate limiter key (e.g., "ratelimit:ip:1.2.3.4")
-- ARGV[1]: capacity (max requests in window)
-- ARGV[2]: window_seconds (time window in seconds)
-- ARGV[3]: current_time (current timestamp in seconds)
-- Returns: {allowed (1 or 0), remaining_capacity}

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

-- Calculate the start of the sliding window
local window_start = now - window

-- Remove expired entries (older than window_start)
-- Using ZREMRANGEBYSCORE for efficient removal by score (timestamp)
redis.call('ZREMRANGEBYSCORE', key, 0, window_start)

-- Count current requests in the window
local current_count = redis.call('ZCARD', key)

local allowed = 0
local remaining = capacity - current_count

-- Check if we're under the limit
if current_count < capacity then
    -- Add current request with timestamp as score and unique ID as member
    -- Using timestamp + random to avoid collision (sorted sets need unique members)
    local member = now .. ':' .. redis.call('INCR', key .. ':counter')
    redis.call('ZADD', key, now, member)
    allowed = 1
    remaining = remaining - 1
end

-- Set expiry to cleanup old keys
-- Adding some buffer to window to ensure we don't lose data prematurely
redis.call('EXPIRE', key, window + 10)
redis.call('EXPIRE', key .. ':counter', window + 10)

return {allowed, math.max(0, remaining)}

