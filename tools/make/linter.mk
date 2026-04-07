##@ Linter

.PHONY: lint
lint: ## Check files
lint: codespell-check markdown-lint

.PHONY: codespell-check
codespell-check: CODESPELL_SKIP := $(shell cat tools/linter/codespell/.codespell.skip | tr \\n ',')
codespell-check: ## Lint check the code-spell
	@$(LOG_TARGET)
	codespell --version
	codespell --skip "$(CODESPELL_SKIP)" --ignore-words ./tools/linter/codespell/.codespell.ignorewords

.PHONY: markdown-lint
markdown-lint: ## Lint check the markdown files.
	@$(LOG_TARGET)
	markdownlint --version
	markdownlint --config ./tools/linter/markdownlint/markdown_lint_config.yaml .
