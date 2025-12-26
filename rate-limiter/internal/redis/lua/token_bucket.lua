-- Token Bucket Rate Limiter
-- KEYS[1]: rate limiter key (e.g., "ratelimit:user:123")
-- ARGV[1]: capacity (max tokens)
-- ARGV[2]: refill_rate (tokens per second)
-- ARGV[3]: current_time_ms (current timestamp in milliseconds)
-- Returns: {allowed (1 or 0), remaining_tokens}

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

-- Get current bucket state
local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

-- First request for this key - initialize the bucket
if tokens == nil then
    tokens = capacity
    last_refill = now
end

-- Calculate tokens to add based on elapsed time
-- Using milliseconds for precision, dividing by 1000 to get seconds
local elapsed_seconds = (now - last_refill) / 1000.0
local tokens_to_add = elapsed_seconds * refill_rate

-- Add tokens but don't exceed capacity
tokens = math.min(capacity, tokens + tokens_to_add)
last_refill = now

-- Check if we can allow this request
local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end

-- Persist the updated state
-- Using HMSET for atomic update of multiple fields
redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)

-- Set expiry to cleanup old keys (2x the time to fill bucket from empty)
-- This prevents memory leaks from inactive keys
local ttl = math.ceil(capacity / refill_rate * 2)
redis.call('EXPIRE', key, ttl)

return {allowed, math.floor(tokens)}

