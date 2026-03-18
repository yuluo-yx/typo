.PHONY: build build-all install test lint fmt clean coverage benchmark help

BINARY_NAME := typo
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := bin
GO := go

# Supported platforms
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none") -X main.date=$(shell date -u +%Y-%m-%d 2>/dev/null || echo "unknown")

help:
	@echo "Typo - Command auto-correction tool"
	@echo ""
	@echo "Usage:"
	@echo "  make build              Build for current platform"
	@echo "  make build-all          Build for all platforms"
	@echo "  make build-linux-amd64  Build for Linux AMD64"
	@echo "  make build-linux-arm64  Build for Linux ARM64"
	@echo "  make build-darwin-amd64 Build for macOS AMD64"
	@echo "  make build-darwin-arm64 Build for macOS ARM64"
	@echo "  make build-windows      Build for Windows AMD64"
	@echo "  make install            Install to GOPATH/bin"
	@echo "  make test               Run tests"
	@echo "  make coverage           Run tests with coverage"
	@echo "  make benchmark          Run benchmarks"
	@echo "  make lint               Run golangci-lint"
	@echo "  make fmt                Format code"
	@echo "  make clean              Clean build artifacts"
	@echo "  make ci                 Run CI checks (fmt, lint, test)"

build:
	@echo "Building $(BINARY_NAME) ($(shell go env GOOS)/$(shell go env GOARCH))..."
	@mkdir -p $(BUILD_DIR)/$(shell go env GOOS)-$(shell go env GOARCH)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(shell go env GOOS)-$(shell go env GOARCH)/$(BINARY_NAME) ./cmd/typo

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows

build-linux-amd64:
	@echo "Building linux/amd64..."
	@mkdir -p $(BUILD_DIR)/linux-amd64
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) ./cmd/typo

build-linux-arm64:
	@echo "Building linux/arm64..."
	@mkdir -p $(BUILD_DIR)/linux-arm64
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) ./cmd/typo

build-darwin-amd64:
	@echo "Building darwin/amd64..."
	@mkdir -p $(BUILD_DIR)/darwin-amd64
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/darwin-amd64/$(BINARY_NAME) ./cmd/typo

build-darwin-arm64:
	@echo "Building darwin/arm64..."
	@mkdir -p $(BUILD_DIR)/darwin-arm64
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/darwin-arm64/$(BINARY_NAME) ./cmd/typo

build-windows:
	@echo "Building windows/amd64..."
	@mkdir -p $(BUILD_DIR)/windows-amd64
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe ./cmd/typo

install:
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install -ldflags="$(LDFLAGS)" ./cmd/typo

test:
	@echo "Running tests..."
	$(GO) test ./... -v -race

coverage:
	@echo "Running tests with coverage..."
	$(GO) test ./... -race -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | tail -1

lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	gofmt -s -w .

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

ci: fmt lint test
	@echo "CI checks completed"

benchmark:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./benchmarks/ -run=^$
