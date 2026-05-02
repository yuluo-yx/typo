##@ Tools

# check and install tools, include GHA and local env.
GOLANGCI_LINT_VERSION ?= v2.11.4
GOLANGCI_LINT_VERSION_NUMBER := $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))

.PHONY: install-tools
install-tools: ## Install tools
install-tools: install-precommit install-golangcilint install-markdownlint install-codespell

.PHONY: install-golangcilint
install-golangcilint: ## Install golangci-lint
	@$(LOG_TARGET)
	@if command -v golangci-lint >/dev/null 2>&1 && golangci-lint version | grep -Eq "version $(GOLANGCI_LINT_VERSION_NUMBER)([^0-9.]|$$)"; then \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION_NUMBER) is already installed, skipping..."; \
	else \
		curl -sSfL "https://github.com/golangci/golangci-lint/releases/download/$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION_NUMBER)-linux-amd64.tar.gz" | tar xzf - -C /tmp && mv /tmp/golangci-lint-$(GOLANGCI_LINT_VERSION_NUMBER)-linux-amd64/golangci-lint $$(go env GOPATH)/bin/golangci-lint; \
	fi
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
