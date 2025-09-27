# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=currency-exchange-api
BINARY_UNIX=$(BINARY_NAME)_unix

# Docker parameters
DOCKER_IMAGE=currency-exchange-api
DOCKER_TAG=latest

.PHONY: all build clean test deps run docker-build docker-run docker-stop help

# Default target
all: deps build

# Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...

# Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

# Run integration tests with concurrent load testing
test-integration:
	$(GOTEST) -v -run="TestConcurrent|TestRace|TestStress|TestCache" ./internal/api

# Run race condition tests
test-race:
	$(GOTEST) -v -run="TestRace" ./internal/api

# Run load tests
test-load:
	$(GOTEST) -v -run="TestConcurrent|TestStress" ./internal/api

# Run benchmarks
benchmark:
	$(GOTEST) -bench=. -benchmem ./internal/api

# Run comprehensive test suite
test-comprehensive: test test-integration benchmark

# Build load testing tool
build-loadtest:
	$(GOBUILD) -o loadtest ./cmd/loadtest

# Run load testing tool
run-loadtest: build-loadtest
	./loadtest -url="http://localhost:8081/api/v1/rates" -users=50 -requests=100 -timeout=30s

# Run stress test
run-stress: build-loadtest
	./loadtest -url="http://localhost:8081/api/v1/rates" -users=100 -requests=50 -timeout=60s -duration=5m

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run the application
run:
	$(GOCMD) run main.go

# Run with hot reload (requires air)
dev:
	air

# Docker build
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Docker run
docker-run:
	docker run -p 8080:8080 --env-file env.example $(DOCKER_IMAGE):$(DOCKER_TAG)

# Docker Compose up
up:
	docker-compose up --build

# Docker Compose down
down:
	docker-compose down

# Docker Compose up in background
up-d:
	docker-compose up -d --build

# Format code
fmt:
	$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Security scan (requires gosec)
security:
	gosec ./...

# Install development tools
install-tools:
	$(GOGET) -u github.com/cosmtrek/air
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint
	$(GOGET) -u github.com/securecodewarrior/gosec/v2/cmd/gosec

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Download deps and build"
	@echo "  build        - Build the application"
	@echo "  build-linux  - Build for Linux"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  test-integration - Run integration tests with concurrent load testing"
	@echo "  test-race    - Run race condition tests"
	@echo "  test-load    - Run load tests"
	@echo "  benchmark    - Run benchmarks"
	@echo "  test-comprehensive - Run comprehensive test suite"
	@echo "  build-loadtest - Build load testing tool"
	@echo "  run-loadtest - Run load testing tool"
	@echo "  run-stress   - Run stress test"
	@echo "  deps         - Download dependencies"
	@echo "  run          - Run the application"
	@echo "  dev          - Run with hot reload (requires air)"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  up           - Start with Docker Compose"
	@echo "  down         - Stop Docker Compose"
	@echo "  up-d         - Start Docker Compose in background"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code (requires golangci-lint)"
	@echo "  security     - Security scan (requires gosec)"
	@echo "  install-tools- Install development tools"
	@echo "  help         - Show this help"



