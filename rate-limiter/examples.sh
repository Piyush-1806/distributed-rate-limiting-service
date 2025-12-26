#!/bin/bash

# Quick start examples for testing the rate limiter

echo "=== Rate Limiter Examples ==="
echo ""

# Wait for service to be ready
echo "Waiting for service..."
sleep 2

BASE_URL="http://localhost:8080"

echo "1. Testing Token Bucket (10 capacity, 1 token/sec)"
echo "   Making 12 requests rapidly..."
for i in {1..12}; do
  response=$(curl -s -X POST $BASE_URL/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "user:alice",
      "algorithm": "token_bucket",
      "capacity": 10,
      "refill_rate": 1
    }')
  allowed=$(echo $response | grep -o '"allowed":[^,}]*' | cut -d':' -f2)
  remaining=$(echo $response | grep -o '"remaining":[^,}]*' | cut -d':' -f2)
  echo "   Request $i: allowed=$allowed, remaining=$remaining"
done

echo ""
echo "2. Testing Sliding Window (5 requests per 10 seconds)"
echo "   Making 7 requests..."
for i in {1..7}; do
  response=$(curl -s -X POST $BASE_URL/check \
    -H "Content-Type: application/json" \
    -d '{
      "key": "ip:192.168.1.1",
      "algorithm": "sliding_window",
      "capacity": 5,
      "window_seconds": 10
    }')
  allowed=$(echo $response | grep -o '"allowed":[^,}]*' | cut -d':' -f2)
  remaining=$(echo $response | grep -o '"remaining":[^,}]*' | cut -d':' -f2)
  echo "   Request $i: allowed=$allowed, remaining=$remaining"
done

echo ""
echo "3. Checking health endpoint"
curl -s $BASE_URL/health | jq '.'

echo ""
echo "4. Fetching metrics (sample)"
curl -s $BASE_URL/metrics | grep "^requests_" | head -5

echo ""
echo "=== Examples Complete ==="

