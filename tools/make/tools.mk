##@ Tools

# check and install tools, include ci and style etc.

.PHONY: install-golanglint
install-golanglint: ## Install golangci-lint
	@$(LOG_TARGET)
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.11.3
	@golangci-lint --version
