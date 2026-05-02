##@ typo build

# Supported platforms
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

CMD_PACKAGE := github.com/yuluo-yx/typo/internal/cmd
LDFLAGS := -s -w -X $(CMD_PACKAGE).version=$(VERSION) -X $(CMD_PACKAGE).commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none") -X $(CMD_PACKAGE).date=$(shell date -u +%Y-%m-%d 2>/dev/null || echo "unknown")

.PHONY: build
build: ## Build the binary for the current platform
	@$(LOG_TARGET)
	@mkdir -p $(BUILD_DIR)/$(shell go env GOOS)-$(shell go env GOARCH)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(shell go env GOOS)-$(shell go env GOARCH)/$(BINARY_NAME) ./cmd/typo

.PHONY: verify-version-metadata
verify-version-metadata: ## Verify build-time version metadata is injected into the binary
	@$(LOG_TARGET)
	@tmp_dir="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	$(GO) build -ldflags="$(LDFLAGS)" -o "$$tmp_dir/$(BINARY_NAME)" ./cmd/typo; \
	output="$$("$$tmp_dir/$(BINARY_NAME)" version)"; \
	case "$$output" in \
		*"typo $(VERSION) "*) ;; \
		*) echo "expected version output to contain 'typo $(VERSION) ', got: $$output" >&2; exit 1 ;; \
	esac

.PHONY: install
install: ## Install the binary to GOPATH/bin
	@$(LOG_TARGET)
	$(GO) install -ldflags="$(LDFLAGS)" ./cmd/typo

.PHONY: build-all
build-all: ## Build for all supported platforms
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-windows-arm64

.PHONY: build-linux-amd64
build-linux-amd64: ## build typo for linux/amd64
	@$(LOG_TARGET)
	@mkdir -p $(BUILD_DIR)/linux-amd64
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) ./cmd/typo

.PHONY: build-linux-arm64
build-linux-arm64: ## build typo for linux/arm64
	@$(LOG_TARGET)
	@mkdir -p $(BUILD_DIR)/linux-arm64
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/linux-arm64/$(BINARY_NAME) ./cmd/typo

.PHONY: build-darwin-amd64
build-darwin-amd64: ## build typo for darwin/amd64
	@$(LOG_TARGET)
	@mkdir -p $(BUILD_DIR)/darwin-amd64
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/darwin-amd64/$(BINARY_NAME) ./cmd/typo

.PHONY: build-darwin-arm64
build-darwin-arm64: ## build typo for darwin/arm64
	@$(LOG_TARGET)
	@mkdir -p $(BUILD_DIR)/darwin-arm64
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/darwin-arm64/$(BINARY_NAME) ./cmd/typo

.PHONY: build-windows-amd64
build-windows-amd64: ## build typo for windows/amd64
	@$(LOG_TARGET)
	@mkdir -p $(BUILD_DIR)/windows-amd64
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/windows-amd64/$(BINARY_NAME).exe ./cmd/typo

.PHONY: build-windows-arm64
build-windows-arm64: ## build typo for windows/arm64
	@$(LOG_TARGET)
	@mkdir -p $(BUILD_DIR)/windows-arm64
	GOOS=windows GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/windows-arm64/$(BINARY_NAME).exe ./cmd/typo

##@ test

.PHONY: download
download: ## Download dependencies
	@$(LOG_TARGET)
	$(GO) mod download

.PHONY: fmt
fmt: ## Run go fmt
	@$(LOG_TARGET)
	$(GO) fmt ./...
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Check go fmt without modifying files
	@$(LOG_TARGET)
	@files="$$(find . -type f -name '*.go' -not -path './.git/*')"; \
	unformatted="$$(gofmt -s -l $$files)"; \
	if [ -n "$$unformatted" ]; then \
		echo "$$unformatted"; \
		echo "go files are not formatted; run make fmt" >&2; \
		exit 1; \
	fi

.PHONY: test
test: ## Run project test
	@$(LOG_TARGET)
	$(GO) test ./... -v -race

.PHONY: ci
ci: ## Run CI-aligned checks for formatting, linting, spelling, and tests
ci: ci-tools fmt-check lint-go codespell-check test

.PHONY: lint-go
lint-go: ## Run golangci-lint
	@$(LOG_TARGET)
	@golangci-lint version | grep -Eq "version $(GOLANGCI_LINT_VERSION_NUMBER)([^0-9.]|$$)" || (echo "golangci-lint $(GOLANGCI_LINT_VERSION) is required; run make install-golangcilint"; exit 1)
	golangci-lint run ./... --config tools/linter/go/.golangci.yml

.PHONY: clean
clean: ## Clean build artifacts
	@$(LOG_TARGET)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

.PHONY: coverage
coverage: ## Run tests with coverage
	@$(LOG_TARGET)
	$(GO) test ./... -race -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: coverage-check
coverage-check: ## Run tests and fail when total coverage is below 90%
	@$(LOG_TARGET)
	$(GO) test ./... -race -coverprofile=coverage.out -covermode=atomic
	@coverage="$$($(GO) tool cover -func=coverage.out | awk '/^total:/ { sub(/%/, "", $$3); print $$3 }')"; \
	echo "total coverage: $$coverage%"; \
	awk -v coverage="$$coverage" 'BEGIN { exit !(coverage + 0 >= 90.0) }' || { \
		echo "total coverage must be at least 90.0%" >&2; \
		exit 1; \
	}

.PHONY: benchmark
benchmark: ## Run benchmarks
	@$(LOG_TARGET)
	$(GO) test -bench=. -benchmem ./benchmarks/ -run=^$
