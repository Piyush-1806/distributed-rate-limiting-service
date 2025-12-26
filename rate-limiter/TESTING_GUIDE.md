# Complete Testing Guide - Distributed Rate Limiter

This guide provides step-by-step instructions to test all functionalities of the distributed rate limiting service without Docker.

---

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Setup](#setup)
3. [Starting the Service](#starting-the-service)
4. [Test 1: Health Check](#test-1-health-check)
5. [Test 2: Token Bucket - Basic](#test-2-token-bucket---basic)
6. [Test 3: Token Bucket - Burst Handling](#test-3-token-bucket---burst-handling)
7. [Test 4: Token Bucket - Refill Mechanism](#test-4-token-bucket---refill-mechanism)
8. [Test 5: Sliding Window - Basic](#test-5-sliding-window---basic)
9. [Test 6: Sliding Window - Time Window Reset](#test-6-sliding-window---time-window-reset)
10. [Test 7: Concurrent Requests (Atomicity)](#test-7-concurrent-requests-atomicity)
11. [Test 8: Multiple Users (Isolation)](#test-8-multiple-users-isolation)
12. [Test 9: Redis Data Inspection](#test-9-redis-data-inspection)
13. [Test 10: Prometheus Metrics](#test-10-prometheus-metrics)
14. [Test 11: Error Handling (Fail-Open)](#test-11-error-handling-fail-open)
15. [Test 12: Performance Testing](#test-12-performance-testing)
16. [Cleanup](#cleanup)

---

## Prerequisites

### Required Software
```bash
# Check Go installation
go version
# Required: Go 1.21 or higher

# Check Redis installation
redis-cli --version
# Required: Redis 6.0 or higher

# Optional but recommended
jq --version  # For pretty JSON output
```

### Install Missing Dependencies

**macOS:**
```bash
# Install Go
brew install go

# Install Redis
brew install redis

# Install jq
brew install jq
```

**Linux (Ubuntu/Debian):**
```bash
# Install Go
sudo apt update
sudo apt install golang-go

# Install Redis
sudo apt install redis-server

# Install jq
sudo apt install jq
```

---

## Setup

### Step 1: Start Redis

**macOS (Homebrew):**
```bash
# Start Redis as a service
brew services start redis

# OR run in foreground
redis-server
```

**Linux:**
```bash
# Start Redis service
sudo systemctl start redis-server

# Check status
sudo systemctl status redis-server

# OR run in foreground
redis-server
```

### Step 2: Verify Redis is Running

```bash
redis-cli ping
```

**Expected Output:**
```
PONG
```

If you don't see `PONG`, Redis is not running. Troubleshoot before continuing.

### Step 3: Navigate to Project Directory

```bash
cd "/path/to/Cloud-Native Distributed Rate Limiting Service/rate-limiter"
```

### Step 4: Install Go Dependencies

```bash
go mod download
go mod tidy
```

**Expected Output:**
```
go: downloading github.com/go-redis/redis/v8 v8.11.5
go: downloading github.com/prometheus/client_golang v1.17.0
...
```

---

## Starting the Service

### Terminal 1: Start the Rate Limiter Service

```bash
go run cmd/server/main.go
```

**Expected Output:**
```
2025/12/27 01:55:39 Starting Rate Limiter Service...
2025/12/27 01:55:39 Config loaded: Redis=localhost:6379, Port=8080
2025/12/27 01:55:39 Redis connection established successfully
2025/12/27 01:55:39 ✅ Redis connected successfully
2025/12/27 01:55:39 Server listening on port 8080
```

**✅ Success Indicator:** You see "Server listening on port 8080"

**❌ If Service Fails to Start:**
- Check if port 8080 is already in use: `lsof -i :8080`
- Check if Redis is running: `redis-cli ping`
- Check Go module errors: `go mod verify`

---

## Test 1: Health Check

### Terminal 2: Open a New Terminal

Test the health endpoint to verify the service is running.

```bash
curl http://localhost:8080/health
```

**Expected Output:**
```json
{"status":"healthy"}
```

### Verify with Pretty Print

```bash
curl -s http://localhost:8080/health | jq '.'
```

**Expected Output:**
```json
{
  "status": "healthy"
}
```

**✅ Test Passed:** Status is "healthy"

---

## Test 2: Token Bucket - Basic

Test basic token bucket rate limiting with 5 tokens and 1 token/second refill rate.

### Make 7 Requests (5 should pass, 2 should be blocked)

```bash
echo "=== Token Bucket: 5 capacity, 1/sec refill ==="
for i in {1..7}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:test1",
      "algorithm": "token_bucket",
      "capacity": 5,
      "refill_rate": 1
    }')
  echo "Request $i: $response"
  sleep 0.1
done
```

**Expected Output:**
```
=== Token Bucket: 5 capacity, 1/sec refill ===
Request 1: {"allowed":true,"remaining":4}
Request 2: {"allowed":true,"remaining":3}
Request 3: {"allowed":true,"remaining":2}
Request 4: {"allowed":true,"remaining":1}
Request 5: {"allowed":true,"remaining":0}
Request 6: {"allowed":false,"remaining":0}
Request 7: {"allowed":false,"remaining":0}
```

**✅ Test Passed:** 
- First 5 requests allowed (true)
- Remaining decreases from 4 to 0
- Requests 6-7 blocked (false)

---

## Test 3: Token Bucket - Burst Handling

Test burst traffic capability with high capacity.

```bash
echo "=== Token Bucket Burst: 20 capacity, 2/sec refill ==="
for i in {1..25}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:burst",
      "algorithm": "token_bucket",
      "capacity": 20,
      "refill_rate": 2
    }')
  allowed=$(echo $response | grep -o '"allowed":[^,}]*' | cut -d':' -f2)
  remaining=$(echo $response | grep -o '"remaining":[^,}]*' | cut -d':' -f2)
  
  if [ "$i" -le 5 ] || [ "$i" -ge 20 ]; then
    echo "Request $i: allowed=$allowed, remaining=$remaining"
  elif [ "$i" -eq 6 ]; then
    echo "... (requests 6-19) ..."
  fi
done
```

**Expected Output:**
```
=== Token Bucket Burst: 20 capacity, 2/sec refill ===
Request 1: allowed=true, remaining=19
Request 2: allowed=true, remaining=18
Request 3: allowed=true, remaining=17
Request 4: allowed=true, remaining=16
Request 5: allowed=true, remaining=15
... (requests 6-19) ...
Request 20: allowed=true, remaining=0
Request 21: allowed=false, remaining=0
Request 22: allowed=false, remaining=0
Request 23: allowed=false, remaining=0
Request 24: allowed=false, remaining=0
Request 25: allowed=false, remaining=0
```

**✅ Test Passed:**
- First 20 requests allowed (burst capacity)
- Requests 21-25 blocked
- Demonstrates burst tolerance

---

## Test 4: Token Bucket - Refill Mechanism

Test that tokens refill over time.

```bash
echo "=== Testing Token Refill ==="

# Use up all tokens
echo "Step 1: Using up all 3 tokens..."
for i in {1..3}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:refill",
      "algorithm": "token_bucket",
      "capacity": 3,
      "refill_rate": 1
    }')
  echo "Request $i: $response"
done

# Try immediately (should fail)
echo -e "\nStep 2: Trying immediately (should be blocked)..."
response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user:refill",
    "algorithm": "token_bucket",
    "capacity": 3,
    "refill_rate": 1
  }')
echo "Request 4 (immediate): $response"

# Wait 2 seconds and try again (should have ~2 tokens refilled)
echo -e "\nStep 3: Waiting 2 seconds for refill..."
sleep 2

response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user:refill",
    "algorithm": "token_bucket",
    "capacity": 3,
    "refill_rate": 1
  }')
echo "Request 5 (after 2s): $response"
```

**Expected Output:**
```
=== Testing Token Refill ===
Step 1: Using up all 3 tokens...
Request 1: {"allowed":true,"remaining":2}
Request 2: {"allowed":true,"remaining":1}
Request 3: {"allowed":true,"remaining":0}

Step 2: Trying immediately (should be blocked)...
Request 4 (immediate): {"allowed":false,"remaining":0}

Step 3: Waiting 2 seconds for refill...
Request 5 (after 2s): {"allowed":true,"remaining":1}
```

**✅ Test Passed:**
- Tokens depleted to 0
- Immediate retry blocked
- After 2 seconds: allowed=true (tokens refilled)

---

## Test 5: Sliding Window - Basic

Test sliding window algorithm with strict rate enforcement.

```bash
echo "=== Sliding Window: 5 requests per 10 seconds ==="
for i in {1..8}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "ip:192.168.1.100",
      "algorithm": "sliding_window",
      "capacity": 5,
      "window_seconds": 10
    }')
  echo "Request $i: $response"
  sleep 0.2
done
```

**Expected Output:**
```
=== Sliding Window: 5 requests per 10 seconds ===
Request 1: {"allowed":true,"remaining":4}
Request 2: {"allowed":true,"remaining":3}
Request 3: {"allowed":true,"remaining":2}
Request 4: {"allowed":true,"remaining":1}
Request 5: {"allowed":true,"remaining":0}
Request 6: {"allowed":false,"remaining":0}
Request 7: {"allowed":false,"remaining":0}
Request 8: {"allowed":false,"remaining":0}
```

**✅ Test Passed:**
- Exactly 5 requests allowed
- All subsequent requests blocked
- Remaining count decreases correctly

---

## Test 6: Sliding Window - Time Window Reset

Test that sliding window resets after time window expires.

```bash
echo "=== Sliding Window Reset Test ==="

# Fill the window
echo "Step 1: Making 3 requests (limit: 3 per 5 seconds)..."
for i in {1..3}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "ip:window-reset",
      "algorithm": "sliding_window",
      "capacity": 3,
      "window_seconds": 5
    }')
  echo "Request $i: $response"
done

# Try immediately (should fail)
echo -e "\nStep 2: Trying 4th request (should be blocked)..."
response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "ip:window-reset",
    "algorithm": "sliding_window",
    "capacity": 3,
    "window_seconds": 5
  }')
echo "Request 4 (immediate): $response"

# Wait for window to slide
echo -e "\nStep 3: Waiting 6 seconds for window to slide..."
sleep 6

echo "Step 4: Making request after window reset..."
response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "ip:window-reset",
    "algorithm": "sliding_window",
    "capacity": 3,
    "window_seconds": 5
  }')
echo "Request 5 (after 6s): $response"
```

**Expected Output:**
```
=== Sliding Window Reset Test ===
Step 1: Making 3 requests (limit: 3 per 5 seconds)...
Request 1: {"allowed":true,"remaining":2}
Request 2: {"allowed":true,"remaining":1}
Request 3: {"allowed":true,"remaining":0}

Step 2: Trying 4th request (should be blocked)...
Request 4 (immediate): {"allowed":false,"remaining":0}

Step 3: Waiting 6 seconds for window to slide...
Step 4: Making request after window reset...
Request 5 (after 6s): {"allowed":true,"remaining":2}
```

**✅ Test Passed:**
- Window enforced correctly
- After 6 seconds (> 5 second window), requests allowed again
- Window "slid" and old entries expired

---

## Test 7: Concurrent Requests (Atomicity)

Test that concurrent requests don't cause race conditions.

```bash
echo "=== Testing Atomicity with Concurrent Requests ==="
echo "Launching 15 concurrent requests (limit: 10)..."

# Launch 15 requests in parallel
for i in {1..15}; do
  (curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:concurrent",
      "algorithm": "token_bucket",
      "capacity": 10,
      "refill_rate": 1
    }' 2>/dev/null) &
done

# Wait for all requests to complete
wait

echo -e "\nCounting allowed vs blocked requests..."
# Make a test request to see final state
response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user:concurrent-verify",
    "algorithm": "token_bucket",
    "capacity": 10,
    "refill_rate": 1
  }')
echo "Verification request: $response"
```

**Expected Behavior:**
- Exactly 10 requests should be allowed
- Remaining 5 should be blocked
- No double-counting or race conditions
- If you see exactly 10 allowed + 5 blocked = atomicity is working

**✅ Test Passed:** No race conditions, exactly 10 allowed

---

## Test 8: Multiple Users (Isolation)

Test that different keys are isolated from each other.

```bash
echo "=== Testing User Isolation ==="

echo "User A - Making 3 requests (limit: 3)..."
for i in {1..3}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:alice",
      "algorithm": "token_bucket",
      "capacity": 3,
      "refill_rate": 1
    }')
  allowed=$(echo $response | grep -o '"allowed":[^,}]*' | cut -d':' -f2)
  echo "  Request $i: allowed=$allowed"
done

echo -e "\nUser B - Making 3 requests (limit: 3)..."
for i in {1..3}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:bob",
      "algorithm": "token_bucket",
      "capacity": 3,
      "refill_rate": 1
    }')
  allowed=$(echo $response | grep -o '"allowed":[^,}]*' | cut -d':' -f2)
  echo "  Request $i: allowed=$allowed"
done

echo -e "\nUser A - 4th request (should be blocked)..."
response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user:alice",
    "algorithm": "token_bucket",
    "capacity": 3,
    "refill_rate": 1
  }')
echo "  $response"

echo -e "\nUser B - 4th request (should be blocked)..."
response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user:bob",
    "algorithm": "token_bucket",
    "capacity": 3,
    "refill_rate": 1
  }')
echo "  $response"
```

**Expected Output:**
```
=== Testing User Isolation ===
User A - Making 3 requests (limit: 3)...
  Request 1: allowed=true
  Request 2: allowed=true
  Request 3: allowed=true

User B - Making 3 requests (limit: 3)...
  Request 1: allowed=true
  Request 2: allowed=true
  Request 3: allowed=true

User A - 4th request (should be blocked)...
  {"allowed":false,"remaining":0}

User B - 4th request (should be blocked)...
  {"allowed":false,"remaining":0}
```

**✅ Test Passed:**
- Each user gets their own independent rate limit
- User A's usage doesn't affect User B
- Both users correctly rate-limited independently

---

## Test 9: Redis Data Inspection

Inspect the actual data structures stored in Redis.

### Create Fresh Rate Limits

```bash
echo "=== Creating Test Data ==="

# Create Token Bucket entry
curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "inspect:token",
    "algorithm": "token_bucket",
    "capacity": 100,
    "refill_rate": 10
  }' | jq '.'

# Create Sliding Window entry
curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "inspect:window",
    "algorithm": "sliding_window",
    "capacity": 50,
    "window_seconds": 60
  }' | jq '.'
```

### Inspect Token Bucket Data

```bash
echo -e "\n=== Token Bucket Data Structure (Hash) ==="
redis-cli HGETALL "inspect:token"
```

**Expected Output:**
```
1) "tokens"
2) "99"
3) "last_refill"
4) "1766780826883"
```

### Inspect Sliding Window Data

```bash
echo -e "\n=== Sliding Window Data Structure (Sorted Set) ==="
redis-cli ZRANGE "inspect:window" 0 -1 WITHSCORES
```

**Expected Output:**
```
1) "1766780826:1"
2) "1766780826"
```

### Check TTL (Time To Live)

```bash
echo -e "\n=== Checking TTL (Auto-Cleanup) ==="
echo "Token Bucket TTL:"
redis-cli TTL "inspect:token"

echo "Sliding Window TTL:"
redis-cli TTL "inspect:window"
```

**Expected Output:**
```
Token Bucket TTL:
20

Sliding Window TTL:
70
```

**✅ Test Passed:**
- Token Bucket uses Hash (HGETALL shows tokens + last_refill)
- Sliding Window uses Sorted Set (ZRANGE shows timestamp entries)
- TTL is set (not -1, meaning auto-cleanup is active)

---

## Test 10: Prometheus Metrics

Verify that metrics are being collected.

### View All Metrics

```bash
curl -s http://localhost:8080/metrics
```

### View Specific Metrics

```bash
echo "=== Request Metrics ==="
curl -s http://localhost:8080/metrics | grep "^requests_"

echo -e "\n=== Redis Metrics ==="
curl -s http://localhost:8080/metrics | grep "^redis_"
```

**Expected Output:**
```
=== Request Metrics ===
requests_allowed_total{algorithm="sliding_window"} 15
requests_allowed_total{algorithm="token_bucket"} 45
requests_blocked_total{algorithm="sliding_window"} 8
requests_blocked_total{algorithm="token_bucket"} 12

=== Redis Metrics ===
redis_errors_total 0
redis_latency_ms_bucket{le="0.1"} 5
redis_latency_ms_bucket{le="0.5"} 58
redis_latency_ms_bucket{le="1"} 80
redis_latency_ms_sum 24.56
redis_latency_ms_count 80
```

### Calculate Average Latency

```bash
echo "=== Calculating Average Redis Latency ==="
metrics=$(curl -s http://localhost:8080/metrics)
sum=$(echo "$metrics" | grep "^redis_latency_ms_sum" | awk '{print $2}')
count=$(echo "$metrics" | grep "^redis_latency_ms_count" | awk '{print $2}')
if [ -n "$sum" ] && [ -n "$count" ] && [ "$count" != "0" ]; then
  avg=$(echo "scale=3; $sum / $count" | bc)
  echo "Average Latency: ${avg}ms"
else
  echo "No metrics available yet"
fi
```

**Expected Output:**
```
=== Calculating Average Redis Latency ===
Average Latency: 0.307ms
```

**✅ Test Passed:**
- Metrics endpoint accessible
- Request counts tracked by algorithm
- Redis latency tracked
- Average latency < 1ms (high performance)

---

## Test 11: Error Handling (Fail-Open)

Test the fail-open behavior when Redis is unavailable.

### Terminal 3: Stop Redis

```bash
# macOS (Homebrew)
brew services stop redis

# Linux
sudo systemctl stop redis-server

# OR if running in foreground, press Ctrl+C in Redis terminal
```

### Verify Redis is Down

```bash
redis-cli ping
```

**Expected Output:**
```
Could not connect to Redis at 127.0.0.1:6379: Connection refused
```

### Make Rate Limit Requests (Should Still Work - Fail-Open)

```bash
echo "=== Testing Fail-Open Behavior ==="
for i in {1..3}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:failopen",
      "algorithm": "token_bucket",
      "capacity": 5,
      "refill_rate": 1
    }')
  echo "Request $i (Redis down): $response"
done
```

**Expected Output:**
```
=== Testing Fail-Open Behavior ===
Request 1 (Redis down): {"allowed":true,"remaining":0}
Request 2 (Redis down): {"allowed":true,"remaining":0}
Request 3 (Redis down): {"allowed":true,"remaining":0}
```

### Check Health Endpoint

```bash
curl -s http://localhost:8080/health | jq '.'
```

**Expected Output:**
```json
{
  "status": "unhealthy",
  "error": "redis connection failed"
}
```

### Check Error Metrics

```bash
curl -s http://localhost:8080/metrics | grep "redis_errors_total"
```

**Expected Output:**
```
redis_errors_total 4
```

**✅ Test Passed:**
- Requests still allowed even with Redis down (fail-open)
- Health endpoint reports unhealthy
- redis_errors_total metric incremented
- Service doesn't crash or block all traffic

### Restart Redis

```bash
# macOS (Homebrew)
brew services start redis

# Linux
sudo systemctl start redis-server

# Verify
redis-cli ping
```

---

## Test 12: Performance Testing

Test system performance and throughput.

### Quick Load Test (100 requests)

```bash
echo "=== Performance Test: 100 Requests ==="
echo "Start time: $(date +%s)"

start=$(date +%s.%N)

for i in {1..100}; do
  curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "perf:test",
      "algorithm": "token_bucket",
      "capacity": 1000,
      "refill_rate": 100
    }' > /dev/null &
done

wait

end=$(date +%s.%N)
duration=$(echo "$end - $start" | bc)

echo "End time: $(date +%s)"
echo "Duration: ${duration}s"
echo "Throughput: $(echo "scale=2; 100 / $duration" | bc) requests/second"
```

**Expected Output:**
```
=== Performance Test: 100 Requests ===
Start time: 1766780950
End time: 1766780952
Duration: 2.34s
Throughput: 42.73 requests/second
```

### Check Performance Metrics

```bash
echo -e "\n=== Performance Metrics ==="
curl -s http://localhost:8080/metrics | grep "redis_latency_ms" | head -5
```

**✅ Test Passed:**
- Requests complete successfully
- Reasonable throughput (>20 req/sec for sequential, >100 req/sec for parallel)
- Average latency < 5ms

---

## Test 13: Invalid Request Handling

Test that the service properly validates requests.

```bash
echo "=== Testing Invalid Requests ==="

echo "1. Missing key:"
curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "algorithm": "token_bucket",
    "capacity": 10,
    "refill_rate": 1
  }' | jq '.'

echo -e "\n2. Invalid algorithm:"
curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "test",
    "algorithm": "invalid_algo",
    "capacity": 10
  }' | jq '.'

echo -e "\n3. Negative capacity:"
curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "test",
    "algorithm": "token_bucket",
    "capacity": -5,
    "refill_rate": 1
  }' | jq '.'

echo -e "\n4. Missing refill_rate for token_bucket:"
curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "test",
    "algorithm": "token_bucket",
    "capacity": 10
  }' | jq '.'

echo -e "\n5. Missing window_seconds for sliding_window:"
curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "test",
    "algorithm": "sliding_window",
    "capacity": 10
  }' | jq '.'
```

**Expected Output:**
```
=== Testing Invalid Requests ===
1. Missing key:
{
  "error": "key is required"
}

2. Invalid algorithm:
{
  "error": "algorithm must be 'token_bucket' or 'sliding_window'"
}

3. Negative capacity:
{
  "error": "capacity must be positive"
}

4. Missing refill_rate for token_bucket:
{
  "error": "refill_rate must be positive for token_bucket"
}

5. Missing window_seconds for sliding_window:
{
  "error": "window_seconds must be positive for sliding_window"
}
```

**✅ Test Passed:**
- All invalid requests properly rejected
- Clear error messages returned
- Service doesn't crash on bad input

---

## Test 14: Different Rate Limit Scenarios

Test real-world use cases.

### Scenario 1: API Rate Limiting (100 req/min)

```bash
echo "=== Scenario 1: API Rate Limit (100/minute) ==="
for i in {1..105}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "api:user:12345",
      "algorithm": "token_bucket",
      "capacity": 100,
      "refill_rate": 1.67
    }')
  if [ "$i" -le 3 ] || [ "$i" -ge 99 ]; then
    allowed=$(echo $response | grep -o '"allowed":[^,}]*' | cut -d':' -f2)
    echo "Request $i: allowed=$allowed"
  elif [ "$i" -eq 4 ]; then
    echo "... (continuing) ..."
  fi
done
```

### Scenario 2: DDoS Protection (10 req/minute per IP)

```bash
echo -e "\n=== Scenario 2: DDoS Protection (10/minute) ==="
for i in {1..15}; do
  response=$(curl -s -X POST http://localhost:8080/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "ip:suspicious:10.0.0.5",
      "algorithm": "sliding_window",
      "capacity": 10,
      "window_seconds": 60
    }')
  allowed=$(echo $response | grep -o '"allowed":[^,}]*' | cut -d':' -f2)
  remaining=$(echo $response | grep -o '"remaining":[^,}]*' | cut -d':' -f2)
  echo "Request $i: allowed=$allowed, remaining=$remaining"
done
```

### Scenario 3: Free Tier Limit (1000 req/day)

```bash
echo -e "\n=== Scenario 3: Free Tier (1000/day) ==="
response=$(curl -s -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user:free:abc123",
    "algorithm": "sliding_window",
    "capacity": 1000,
    "window_seconds": 86400
  }')
echo "Sample request: $response"
```

**✅ Test Passed:**
- Different rate limit configurations work
- Can model various real-world scenarios
- Token bucket for burst, sliding window for strict limits

---

## Test 15: Service Logs

Check the service logs for any errors or warnings.

### View Logs in Terminal 1

Look at the terminal where you started the service. You should see logs like:

```
2025/12/27 01:55:39 Starting Rate Limiter Service...
2025/12/27 01:55:39 Config loaded: Redis=localhost:6379, Port=8080
2025/12/27 01:55:39 Redis connection established successfully
2025/12/27 01:55:39 ✅ Redis connected successfully
2025/12/27 01:55:39 Server listening on port 8080
```

**✅ Test Passed:**
- No error messages
- Service started successfully
- All requests handled without crashes

---

## Summary Report

Generate a final test summary:

```bash
echo "╔════════════════════════════════════════════════════════════╗"
echo "║           TESTING SUMMARY REPORT                           ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "Final Metrics:"
curl -s http://localhost:8080/metrics | grep "^requests_"
echo ""
echo "Redis Performance:"
curl -s http://localhost:8080/metrics | grep "redis_latency_ms_sum\|redis_latency_ms_count\|redis_errors_total"
echo ""
echo "Health Status:"
curl -s http://localhost:8080/health | jq '.'
echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                  ALL TESTS COMPLETE                        ║"
echo "╚════════════════════════════════════════════════════════════╝"
```

---

## Cleanup

### Stop the Service

In Terminal 1 (where service is running):
```bash
# Press Ctrl+C
```

### Clear Redis Data (Optional)

```bash
# Clear all test keys
redis-cli FLUSHDB

# Verify
redis-cli DBSIZE
```

**Expected Output:**
```
(integer) 0
```

### Stop Redis (Optional)

```bash
# macOS
brew services stop redis

# Linux
sudo systemctl stop redis-server
```

---

## Troubleshooting

### Issue: "Connection refused" when accessing service

**Solution:**
```bash
# Check if service is running
lsof -i :8080

# Check for port conflicts
lsof -i :8080 | grep LISTEN

# Try different port
PORT=8081 go run cmd/server/main.go
```

### Issue: Redis connection errors

**Solution:**
```bash
# Check Redis status
redis-cli ping

# Check Redis is listening
lsof -i :6379

# Restart Redis
brew services restart redis  # macOS
sudo systemctl restart redis-server  # Linux
```

### Issue: "command not found: jq"

**Solution:**
```bash
# Install jq
brew install jq  # macOS
sudo apt install jq  # Linux

# OR parse JSON without jq
curl -s http://localhost:8080/health
```

### Issue: Slow performance

**Solution:**
```bash
# Check Redis latency
redis-cli --latency

# Check system resources
top

# Reduce concurrent requests in tests
```

### Issue: Go module errors

**Solution:**
```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download
go mod tidy

# Verify modules
go mod verify
```

---

## Next Steps

After completing all tests:

1. ✅ **Review Metrics** - Understand the performance characteristics
2. ✅ **Modify Parameters** - Try different capacity/rate configurations
3. ✅ **Stress Test** - Use tools like `ab` or `wrk` for heavy load
4. ✅ **Explore Code** - Read the implementation in `internal/` directory
5. ✅ **Deploy to K8s** - Try the Kubernetes deployment (see README.md)

---

## Additional Resources

- **README.md** - Project overview and architecture
- **PROJECT_SUMMARY.md** - Technical deep-dive
- **INTERVIEW_GUIDE.md** - Interview preparation
- **Redis Lua Scripts** - `internal/redis/lua/*.lua`
- **Go Implementation** - `internal/limiter/*.go`

---

**Happy Testing! 🚀**

