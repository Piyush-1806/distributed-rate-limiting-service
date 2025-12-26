# Distributed Rate Limiting Service

A production-grade, cloud-native rate limiter built in Go with Redis. Supports Token Bucket and Sliding Window algorithms with atomic operations guaranteed by Lua scripts.

## Problem Statement

In distributed systems, rate limiting is essential to:
- Prevent API abuse and DoS attacks
- Ensure fair resource usage across users
- Protect downstream services from overload
- Maintain SLA guarantees

Traditional rate limiters using in-memory state don't work in distributed environments where multiple service instances need to share rate limit counters. This service solves that by using Redis as a shared, atomic state store.

## Architecture

```
┌─────────┐
│ Client  │
└────┬────┘
     │
┌────▼────────────┐
│ Load Balancer   │
└────┬────────────┘
     │
     ├──────┬──────┬──────┐
     │      │      │      │
┌────▼──┐ ┌─▼────┐ ┌▼────┐
│ RL-1  │ │ RL-2 │ │ RL-3│  (Stateless rate limiter pods)
└───┬───┘ └──┬───┘ └─┬───┘
    │        │       │
    └────────┼───────┘
             │
        ┌────▼────┐
        │  Redis  │ (Shared state)
        └─────────┘
```

**Key Design Principles:**
- **Stateless service**: All state lives in Redis, allowing horizontal scaling
- **Atomic operations**: Lua scripts ensure race-free rate limiting under concurrency
- **Fail-open strategy**: Service allows requests if Redis is unavailable (prevents cascading failures)
- **Low latency**: Single Redis round-trip per request, target <2ms

## Algorithms

### Token Bucket
Best for: APIs that allow occasional bursts while maintaining average rate

**How it works:**
- Bucket has a capacity (max tokens)
- Tokens refill at a constant rate
- Each request consumes 1 token
- Allows bursts up to capacity

**Example:** 10 tokens capacity, 1 token/sec refill
- User can make 10 requests instantly (burst)
- Then limited to 1 request/sec sustained

**Use case:** User-facing APIs where occasional bursts are acceptable

### Sliding Window Log
Best for: Strict rate enforcement without boundary exploits

**How it works:**
- Tracks timestamp of each request in a sorted set
- Counts requests in rolling time window
- No fixed window boundaries (prevents gaming at edges)

**Example:** 100 requests per 60 seconds
- At any point, checks last 60 seconds of history
- More accurate than fixed windows

**Use case:** Critical APIs requiring precise rate control, preventing abuse

## Atomicity Guarantee

All rate limit checks execute in a single Lua script on Redis:

```lua
-- Read current state
-- Calculate new state
-- Check if allowed
-- Update state
-- Return result
```

This happens **atomically** - no race conditions even under high concurrency. No distributed locks needed.

## Failure Handling

### Fail-Open Strategy
When Redis is unreachable or times out, the service **allows the request**.

**Why fail-open?**
- Prevents rate limiter from becoming a single point of failure
- During incidents, prefer serving traffic over strict rate enforcement
- Monitored via `redis_errors_total` metric - alerts trigger on spikes

**Tradeoffs:**
- Brief periods without rate limiting during Redis outages
- In practice, better than blocking all traffic

**Alternative**: Fail-closed would reject requests, but risks cascading failures.

## API Usage

### Check Rate Limit

```bash
curl -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "user:123",
    "algorithm": "token_bucket",
    "capacity": 10,
    "refill_rate": 1
  }'
```

Response:
```json
{
  "allowed": true,
  "remaining": 9
}
```

### Sliding Window Example

```bash
curl -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "key": "ip:1.2.3.4",
    "algorithm": "sliding_window",
    "capacity": 100,
    "window_seconds": 60
  }'
```

### Health Check

```bash
curl http://localhost:8080/health
```

### Metrics

```bash
curl http://localhost:8080/metrics
```

Key metrics:
- `requests_allowed_total{algorithm="token_bucket"}` - Allowed requests
- `requests_blocked_total{algorithm="sliding_window"}` - Blocked requests
- `redis_latency_ms` - Redis operation latency (histogram)
- `redis_errors_total` - Redis failures triggering fail-open

## Local Development

### Prerequisites
- Go 1.21+
- Redis 6+

### Run Redis
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

### Run Service
```bash
cd rate-limiter
go mod download
go run cmd/server/main.go
```

Service starts on port 8080.

### Configuration

Environment variables:
```bash
PORT=8080                    # Server port
REDIS_ADDR=localhost:6379    # Redis address
REDIS_PASSWORD=              # Redis password
REDIS_DB=0                   # Redis database
REDIS_POOL_SIZE=100          # Connection pool size
REDIS_MIN_IDLE_CONNS=10      # Min idle connections
REDIS_TIMEOUT=2ms            # Redis operation timeout
DEBUG_LOGGING=false          # Enable verbose logging
```

## Docker

### Build Image
```bash
docker build -t rate-limiter:latest .
```

### Run Container
```bash
docker run -d \
  -p 8080:8080 \
  -e REDIS_ADDR=host.docker.internal:6379 \
  rate-limiter:latest
```

## Kubernetes Deployment

### ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rate-limiter-config
data:
  REDIS_ADDR: "redis-service:6379"
  REDIS_POOL_SIZE: "200"
  REDIS_TIMEOUT: "2ms"
```

### Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rate-limiter
spec:
  replicas: 3
  selector:
    matchLabels:
      app: rate-limiter
  template:
    metadata:
      labels:
        app: rate-limiter
    spec:
      containers:
      - name: rate-limiter
        image: rate-limiter:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: rate-limiter-config
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
```

### Service
```yaml
apiVersion: v1
kind: Service
metadata:
  name: rate-limiter-service
spec:
  selector:
    app: rate-limiter
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

## Scaling Strategy

### Horizontal Scaling
- Service is completely stateless
- Add more pods to handle increased load
- Redis handles concurrency via Lua scripts
- No coordination needed between pods

### Redis Scaling
For very high throughput:
1. **Redis Cluster**: Shard keys across multiple Redis nodes
2. **Read Replicas**: Offload health checks to replicas
3. **Redis Sentinel**: High availability with automatic failover

### Performance Tuning
- Increase `REDIS_POOL_SIZE` if seeing pool exhaustion
- Monitor `redis_latency_ms` p99 - should stay <2ms
- Use pipelining if batching multiple checks (future enhancement)

## Performance Characteristics

**Latency:** <2ms per check (single Redis call)
**Throughput:** 10k+ req/sec per instance (depends on Redis)
**Memory:** ~50MB per instance + Redis memory for rate limit state

**Bottleneck:** Redis throughput (typically 100k+ ops/sec on standard instance)

## Testing in Production

### Load Testing
```bash
# Using Apache Bench
ab -n 10000 -c 100 -p payload.json -T application/json \
  http://localhost:8080/check
```

### Verify Atomicity
Run concurrent requests with same key - rate limits should be enforced correctly without double-counting or race conditions.

### Failure Testing
Stop Redis and verify:
- Requests are allowed (fail-open)
- `redis_errors_total` metric increments
- Logs show Redis connection errors

## Design Decisions

**Why Lua scripts over distributed locks?**
- Locks add latency and complexity
- Risk of deadlocks if client crashes
- Lua scripts are atomic by design

**Why fail-open?**
- Rate limiter shouldn't bring down the entire system
- Better to have brief unprotected periods than block all traffic
- Monitored closely via metrics

**Why Redis over in-memory?**
- Distributed systems need shared state
- Redis provides atomic operations
- Battle-tested and widely deployed

**Why net/http over frameworks?**
- Minimal dependencies, easier to understand
- Full control over request handling
- Interview-friendly - no framework magic

## Future Enhancements

- [ ] Distributed tracing (OpenTelemetry)
- [ ] Request batching for higher throughput
- [ ] Fixed window counter algorithm (lighter weight)
- [ ] Configurable fail-closed mode
- [ ] Admin API to view/reset rate limits
- [ ] Redis Cluster support

---

Built for production use and technical interviews. Every design choice can be explained on a whiteboard.

