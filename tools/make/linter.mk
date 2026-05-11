##@ Linter

.PHONY: lint
lint: ## Check files
lint: codespell-check markdown-lint

.PHONY: codespell-check
codespell-check: CODESPELL_SKIP := $(shell cat tools/linter/codespell/.codespell.skip | tr \\n ',')
codespell-check: ## Lint check the code-spell
codespell-check: install-codespell
	@$(LOG_TARGET)
	"$(PYTHON_VENV_BIN)/codespell" --version
	@files="$$(git ls-files --cached --others --exclude-standard)"; \
	"$(PYTHON_VENV_BIN)/codespell" --skip "$(CODESPELL_SKIP)" --ignore-words ./tools/linter/codespell/.codespell.ignorewords $$files

.PHONY: markdown-lint
markdown-lint: ## Lint check the markdown files.
markdown-lint: install-markdownlint
	@$(LOG_TARGET)
	"$(MARKDOWNLINT)" --version
	"$(MARKDOWNLINT)" --config ./tools/linter/markdownlint/markdown_lint_config.yml --ignore tools/node/node_modules .
