##@ Tools

# check and install tools, include GHA and local env.

.PHONY: install-golanglint
install-golanglint: ## Install golangci-lint
	@$(LOG_TARGET)
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@main
	@golangci-lint --version
