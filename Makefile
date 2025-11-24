.PHONY: help setup test lint fmt vet build clean run-postgres run-redis run-scheduler

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: ## Download and tidy dependencies
	go mod download
	go mod tidy

test: ## Run tests (excludes examples)
	go test -v -race -coverprofile=coverage.txt -covermode=atomic $$(go list ./... | grep -v /examples/)

test-unit: ## Run only unit tests (skip integration tests)
	go test -short -v $$(go list ./... | grep -v /examples/)

test-integration: ## Run integration tests (requires PostgreSQL and Redis)
	go test -v ./providers/redis
	go test -v ./providers/postgres

test-all: test-unit test-integration ## Run all tests

docker-test-up: ## Start test infrastructure with Docker Compose
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for services to be ready..."
	@sleep 5

docker-test-down: ## Stop test infrastructure
	docker-compose -f docker-compose.test.yml down -v

lint: ## Run linter
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

build: ## Build all examples
	go build -o bin/postgres-example ./examples/postgres
	go build -o bin/redis-example ./examples/redis
	go build -o bin/scheduler-example ./examples/scheduler

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f go.sum

run-postgres: ## Run PostgreSQL example
	go run ./examples/postgres/main.go

run-redis: ## Run Redis example
	go run ./examples/redis/main.go

run-scheduler: ## Run scheduler example (run multiple instances to see locking)
	go run ./examples/scheduler/main.go

.DEFAULT_GOAL := help

