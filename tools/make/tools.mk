##@ Tools

# check and install tools, include GHA and local env.

.PHONY: install-tools
install-tools: ## Install tools
install-tools: install-precommit install-golangcilint install-markdownlint install-codespell

.PHONY: install-golangcilint
install-golangcilint: ## Install golangci-lint
	@$(LOG_TARGET)
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@golangci-lint --version

.PHONY: install-markdownlint
install-markdownlint: ## Install markdownlint tools
	@$(LOG_TARGET)
	@if command -v markdownlint >/dev/null 2>&1; then \
		echo "markdownlint-cli is already installed, skipping..."; \
	else \
		npm install markdownlint-cli --global; \
	fi

# In local, suggection use python venv.
.PHONY: install-codespell
install-codespell: ## Install codespell tools
	@$(LOG_TARGET)
	@if command -v codespell >/dev/null 2>&1; then \
		echo "codespell is already installed, skipping..."; \
	else \
		pip3 install codespell; \
	fi

.PHONY: install-precommit
install-precommit: ## Install pre-commit hook framework
	@$(LOG_TARGET)
	@if command -v pre-commit >/dev/null 2>&1; then \
		echo "pre-commit is already installed, skipping..."; \
	else \
		pip3 install pre-commit; \
	fi

.PHONY: setup
setup: ## One-time dev environment setup (installs pre-commit and configures hooks)
setup: install-precommit
	@git config core.hooksPath .githooks
	@echo "Done. Pre-commit hooks are now active."
