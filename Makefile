.PHONY: build build-all install test lint fmt clean coverage benchmark help

BINARY_NAME := typo
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := bin
GO := go

# 支持的平台
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none") -X main.date=$(shell date -u +%Y-%m-%d 2>/dev/null || echo "unknown")

help:
	@echo "Typo - 命令快速修正工具"
	@echo ""
	@echo "使用方法:"
	@echo "  make build              编译当前平台二进制文件"
	@echo "  make build-all          编译所有平台二进制文件"
	@echo "  make build-linux-amd64  编译 Linux AMD64"
	@echo "  make build-linux-arm64  编译 Linux ARM64"
	@echo "  make build-darwin-amd64 编译 macOS AMD64"
	@echo "  make build-darwin-arm64 编译 macOS ARM64"
	@echo "  make build-windows      编译 Windows AMD64"
	@echo "  make install            安装到 GOPATH/bin"
	@echo "  make test               运行测试"
	@echo "  make coverage           运行测试并生成覆盖率报告"
	@echo "  make benchmark          运行性能测试"
	@echo "  make lint               运行 golangci-lint"
	@echo "  make fmt                格式化代码"
	@echo "  make clean              清理构建产物"
	@echo "  make ci                 运行 CI 检查 (fmt, lint, test)"

build:
	@echo "构建 $(BINARY_NAME) ($(shell go env GOOS)/$(shell go env GOARCH))..."
	@mkdir -p $(BUILD_DIR)/$(shell go env GOOS)-$(shell go env GOARCH)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(shell go env GOOS)-$(shell go env GOARCH)/$(BINARY_NAME) ./cmd/typo

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows

build-linux-amd64:
	@echo "构建 linux/amd64..."
	@mkdir -p $(BUILD_DIR)/linux-amd64
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) ./cmd/typo

build-linux-arm64:
	@echo "构建 linux/arm64..."
	@mkdir -p $(BUILD_DIR)/linux-arm64
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) ./cmd/typo

build-darwin-amd64:
	@echo "构建 darwin/amd64..."
	@mkdir -p $(BUILD_DIR)/darwin-amd64
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/darwin-amd64/$(BINARY_NAME) ./cmd/typo

build-darwin-arm64:
	@echo "构建 darwin/arm64..."
	@mkdir -p $(BUILD_DIR)/darwin-arm64
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/darwin-arm64/$(BINARY_NAME) ./cmd/typo

build-windows:
	@echo "构建 windows/amd64..."
	@mkdir -p $(BUILD_DIR)/windows-amd64
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe ./cmd/typo

install:
	@echo "安装 $(BINARY_NAME)..."
	$(GO) install -ldflags="$(LDFLAGS)" ./cmd/typo

test:
	@echo "运行测试..."
	$(GO) test ./... -v -race

coverage:
	@echo "运行测试并生成覆盖率报告..."
	$(GO) test ./... -race -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | tail -1

lint:
	@echo "运行 golangci-lint..."
	golangci-lint run ./...

fmt:
	@echo "格式化代码..."
	$(GO) fmt ./...
	gofmt -s -w .

clean:
	@echo "清理..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

ci: fmt lint test
	@echo "CI 检查完成"

benchmark:
	@echo "运行性能测试..."
	$(GO) test -bench=. -benchmem ./benchmarks/ -run=^$
