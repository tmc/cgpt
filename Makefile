# cgpt Makefile
# Provides convenient targets for building, testing, and developing cgpt

# Go configuration
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Build configuration
BINARY_NAME = cgpt
MAIN_PATH = ./cmd/cgpt

# Test configuration
TEST_TIMEOUT = 30m
BENCH_TIME = 10s
INTEGRATION_TEST_PATTERN = ./integration_test.go

# Default target
all: test build

# Build targets
.PHONY: build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PATH)

.PHONY: build-race
build-race:
	$(GOBUILD) -race -o $(BINARY_NAME) -v $(MAIN_PATH)

# Test targets
.PHONY: test
test:
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-short
test-short:
	$(GOTEST) -v -short -timeout 5m ./...

.PHONY: test-race
test-race:
	$(GOTEST) -v -race -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) -run "TestIntegration_" ./...

.PHONY: test-unit
test-unit:
	@echo "Running unit tests (excluding integration tests)..."
	$(GOTEST) -v -timeout 10m -run "^Test[^I]" ./... || \
	$(GOTEST) -v -timeout 10m -run "^Test.*[^Integration]$$" ./...

.PHONY: benchmark
benchmark:
	@echo "Running all benchmarks..."
	$(GOTEST) -v -bench=. -benchtime=$(BENCH_TIME) -timeout $(TEST_TIMEOUT) ./...

.PHONY: benchmark-integration
benchmark-integration:
	@echo "Running integration benchmarks..."
	$(GOTEST) -v -bench="BenchmarkIntegration_" -benchtime=$(BENCH_TIME) -timeout $(TEST_TIMEOUT) ./...

.PHONY: benchmark-completion
benchmark-completion:
	@echo "Running completion performance benchmarks..."
	$(GOTEST) -v -bench="BenchmarkIntegration_Completion" -benchtime=$(BENCH_TIME) -timeout $(TEST_TIMEOUT) ./...

.PHONY: benchmark-history
benchmark-history:
	@echo "Running history operations benchmarks..."
	$(GOTEST) -v -bench="BenchmarkIntegration_History" -benchtime=$(BENCH_TIME) -timeout $(TEST_TIMEOUT) ./...

.PHONY: benchmark-config
benchmark-config:
	@echo "Running configuration parsing benchmarks..."
	$(GOTEST) -v -bench="BenchmarkIntegration_Configuration" -benchtime=$(BENCH_TIME) -timeout $(TEST_TIMEOUT) ./...

# Test coverage
.PHONY: coverage
coverage:
	$(GOTEST) -v -coverprofile=coverage.out -timeout $(TEST_TIMEOUT) ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: coverage-integration
coverage-integration:
	$(GOTEST) -v -coverprofile=coverage-integration.out -timeout $(TEST_TIMEOUT) -run "TestIntegration_" ./...
	$(GOCMD) tool cover -html=coverage-integration.out -o coverage-integration.html
	@echo "Integration test coverage report generated: coverage-integration.html"

# Linting and formatting
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

.PHONY: vet
vet:
	$(GOCMD) vet ./...

.PHONY: lint
lint: fmt vet
	@echo "Linting complete"

# Dependency management
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

.PHONY: deps-update
deps-update:
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Clean targets
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -f coverage-integration.out coverage-integration.html

.PHONY: clean-test-cache
clean-test-cache:
	$(GOCMD) clean -testcache

# Development targets
.PHONY: dev
dev: clean-test-cache lint test-short build

.PHONY: ci
ci: clean-test-cache lint test-race coverage

# Release targets
.PHONY: release-test
release-test: clean-test-cache lint test benchmark

# Install targets
.PHONY: install
install:
	$(GOCMD) install $(MAIN_PATH)

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              - Build the cgpt binary"
	@echo "  build-race         - Build with race detection enabled"
	@echo "  test               - Run all tests"
	@echo "  test-short         - Run tests with short flag"
	@echo "  test-race          - Run tests with race detection"
	@echo "  test-integration   - Run only integration tests"
	@echo "  test-unit          - Run only unit tests"
	@echo "  benchmark          - Run all benchmarks"
	@echo "  benchmark-integration - Run integration benchmarks"
	@echo "  benchmark-completion  - Run completion benchmarks"
	@echo "  benchmark-history     - Run history benchmarks"
	@echo "  benchmark-config      - Run config benchmarks"
	@echo "  coverage           - Generate test coverage report"
	@echo "  coverage-integration - Generate integration test coverage"
	@echo "  lint               - Run linting (fmt + vet)"
	@echo "  deps               - Download and tidy dependencies"
	@echo "  deps-update        - Update all dependencies"
	@echo "  clean              - Clean build artifacts and coverage files"
	@echo "  clean-test-cache   - Clean Go test cache"
	@echo "  dev                - Development workflow (lint + test-short + build)"
	@echo "  ci                 - CI workflow (lint + test-race + coverage)"
	@echo "  release-test       - Pre-release testing (lint + test + benchmark)"
	@echo "  install            - Install cgpt to GOPATH/bin"
	@echo "  help               - Show this help message"