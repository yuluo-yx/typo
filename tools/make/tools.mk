##@ Tools

# Check and install tools for GitHub Actions and local environments.
GOLANGCI_LINT_VERSION ?= v2.11.4
GOLANGCI_LINT_VERSION_NUMBER := $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))
GOVULNCHECK_VERSION ?= v1.3.0
GITLEAKS_VERSION ?= v8.30.1
NODE_TOOLS_DIR ?= tools/node
MARKDOWNLINT := $(NODE_TOOLS_DIR)/node_modules/.bin/markdownlint
PYTHON ?= python3
PYTHON_VENV ?= .venv
PYTHON_VENV_BIN := $(PYTHON_VENV)/bin
CODESPELL_VERSION ?= 2.4.2
PRE_COMMIT_VERSION ?= 4.0.1

.PHONY: install-tools
install-tools: ## Install tools
install-tools: install-precommit install-golangcilint install-markdownlint install-codespell install-govulncheck install-gitleaks

.PHONY: ci-tools
ci-tools: ## Install tools required by make ci
ci-tools: install-golangcilint install-markdownlint install-codespell install-govulncheck install-gitleaks

.PHONY: install-golangcilint
install-golangcilint: ## Install golangci-lint
	@$(LOG_TARGET)
	@if command -v golangci-lint >/dev/null 2>&1 && golangci-lint version | grep -Eq "version $(GOLANGCI_LINT_VERSION_NUMBER)([^0-9.]|$$)"; then \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION_NUMBER) is already installed, skipping..."; \
	else \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	@golangci-lint --version

.PHONY: install-markdownlint
install-markdownlint: ## Install markdownlint tools
	@$(LOG_TARGET)
	npm install --prefix "$(NODE_TOOLS_DIR)" --no-audit --no-fund --package-lock=false
	@"$(MARKDOWNLINT)" --version

.PHONY: python-venv
python-venv: ## Create the project-local Python virtual environment
	@$(LOG_TARGET)
	@if ! test -x "$(PYTHON_VENV_BIN)/python"; then \
		$(PYTHON) -m venv "$(PYTHON_VENV)"; \
		"$(PYTHON_VENV_BIN)/python" -m pip install --upgrade pip; \
	fi

.PHONY: install-codespell
install-codespell: ## Install codespell tools
install-codespell: python-venv
	@$(LOG_TARGET)
	@"$(PYTHON_VENV_BIN)/python" -m pip install "codespell==$(CODESPELL_VERSION)"
	@"$(PYTHON_VENV_BIN)/codespell" --version

.PHONY: install-govulncheck
install-govulncheck: ## Install govulncheck
	@$(LOG_TARGET)
	@if command -v govulncheck >/dev/null 2>&1; then \
		echo "govulncheck is already installed, skipping..."; \
	else \
		go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); \
	fi
	@govulncheck -version

.PHONY: install-gitleaks
install-gitleaks: ## Install gitleaks
	@$(LOG_TARGET)
	@if command -v gitleaks >/dev/null 2>&1; then \
		echo "gitleaks is already installed, skipping..."; \
	else \
		go install github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION); \
	fi
	@gitleaks version

.PHONY: install-precommit
install-precommit: ## Install pre-commit hook framework
install-precommit: python-venv
	@$(LOG_TARGET)
	@"$(PYTHON_VENV_BIN)/python" -m pip install "pre-commit==$(PRE_COMMIT_VERSION)"
	@"$(PYTHON_VENV_BIN)/pre-commit" --version

.PHONY: setup
setup: ## One-time dev environment setup (installs pre-commit and configures hooks)
setup: install-precommit
	@git config core.hooksPath .githooks
	@echo "Done. Pre-commit hooks are now active."
