.PHONY: help build run test docker-build docker-run clean dev redis-up redis-down

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the rate limiter binary
	@echo "Building rate-limiter..."
	@go build -o rate-limiter ./cmd/server

run: ## Run the service (requires Redis)
	@echo "Starting rate limiter..."
	@go run ./cmd/server/main.go

dev: redis-up run ## Start Redis and run the service

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race ./...

redis-up: ## Start Redis using Docker
	@echo "Starting Redis..."
	@docker run -d --name rate-limiter-redis -p 6379:6379 redis:7-alpine || echo "Redis already running"

redis-down: ## Stop Redis container
	@echo "Stopping Redis..."
	@docker stop rate-limiter-redis && docker rm rate-limiter-redis || echo "Redis not running"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t rate-limiter:latest .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -d \
		-p 8080:8080 \
		-e REDIS_ADDR=host.docker.internal:6379 \
		--name rate-limiter \
		rate-limiter:latest

docker-stop: ## Stop Docker container
	@docker stop rate-limiter && docker rm rate-limiter || echo "Container not running"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f rate-limiter
	@go clean

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run || go vet ./...

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

example: ## Run example requests
	@./examples.sh

