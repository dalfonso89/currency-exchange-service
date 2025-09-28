# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=currency-exchange-api
BINARY_UNIX=$(BINARY_NAME)_unix

.PHONY: all build clean test deps run help

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

# Run unit tests only
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

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
	@echo "  build-loadtest - Build load testing tool"
	@echo "  run-loadtest - Run load testing tool"
	@echo "  run-stress   - Run stress test"
	@echo "  deps         - Download dependencies"
	@echo "  run          - Run the application"
	@echo "  dev          - Run with hot reload (requires air)"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code (requires golangci-lint)"
	@echo "  security     - Security scan (requires gosec)"
	@echo "  install-tools- Install development tools"
	@echo "  help         - Show this help"