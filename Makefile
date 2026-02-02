.PHONY: build build-api build-worker run-api run-worker test test-coverage lint clean docker-build docker-up docker-down load-test load-test-smoke load-test-lifecycle generate-go-client generate-ts-client generate-clients

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary names
API_BINARY=api-server
WORKER_BINARY=worker

# Build flags
LDFLAGS=-ldflags "-w -s"

# Default target
all: build

# Build all binaries
build: build-api build-worker

# Build API server
build-api:
	$(GOBUILD) $(LDFLAGS) -o bin/$(API_BINARY) ./cmd/api-server

# Build worker
build-worker:
	$(GOBUILD) $(LDFLAGS) -o bin/$(WORKER_BINARY) ./cmd/worker

# Run API server locally
run-api:
	$(GOCMD) run ./cmd/api-server

# Run worker locally
run-worker:
	$(GOCMD) run ./cmd/worker

# Run tests
test:
	$(GOTEST) -v -race ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run integration tests
test-integration:
	$(GOTEST) -v -race -tags=integration ./test/integration/...

# Run linter
lint:
	$(GOLINT) run ./...

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Download dependencies
deps:
	$(GOMOD) download

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Development helpers
dev-redis:
	docker run -d --name taskqueue-redis -p 6379:6379 redis:7-alpine

dev-redis-stop:
	docker stop taskqueue-redis && docker rm taskqueue-redis

# Generate mocks (requires mockgen)
mocks:
	go generate ./...

# Install development tools
tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/golang/mock/mockgen@latest

# Load testing (requires k6: brew install k6)
load-test:
	k6 run test/load/task_submission.js

load-test-smoke:
	k6 run --vus 1 --duration 10s test/load/task_submission.js

load-test-lifecycle:
	k6 run test/load/task_lifecycle.js

load-test-stress:
	k6 run --vus 100 --duration 2m test/load/task_submission.js

# Client SDK generation
generate-go-client:
	@echo "Generating Go client from OpenAPI spec..."
	@which oapi-codegen > /dev/null || (echo "Installing oapi-codegen..." && go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest)
	oapi-codegen -generate types,client -package client -o pkg/client/client_gen.go docs/openapi.yaml
	@echo "Go client generated: pkg/client/client_gen.go"

generate-ts-client:
	@echo "Generating TypeScript client from OpenAPI spec..."
	cd clients/typescript && npm install && npm run generate && npm run build
	@echo "TypeScript client generated: clients/typescript/src/generated/"

generate-clients: generate-go-client generate-ts-client
	@echo "All clients generated successfully!"

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build all binaries"
	@echo "  build-api      - Build API server"
	@echo "  build-worker   - Build worker"
	@echo "  run-api        - Run API server locally"
	@echo "  run-worker     - Run worker locally"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-integration - Run integration tests"
	@echo "  lint           - Run linter"
	@echo "  tidy           - Tidy dependencies"
	@echo "  deps           - Download dependencies"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start Docker containers"
	@echo "  docker-down    - Stop Docker containers"
	@echo "  docker-logs    - View Docker logs"
	@echo "  dev-redis      - Start Redis for development"
	@echo "  dev-redis-stop - Stop development Redis"
	@echo "  tools          - Install development tools"
	@echo "  load-test      - Run full k6 load test suite"
	@echo "  load-test-smoke - Run quick smoke test (1 VU, 10s)"
	@echo "  load-test-lifecycle - Run task lifecycle test"
	@echo "  load-test-stress - Run stress test (100 VUs, 2m)"
	@echo "  generate-go-client - Generate Go client from OpenAPI spec"
	@echo "  generate-ts-client - Generate TypeScript client from OpenAPI spec"
	@echo "  generate-clients - Generate all client SDKs"
