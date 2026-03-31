##@ e2e

PHONY: build-e2e-image
build-e2e-image: ## Build the Docker image for end-to-end tests
	@$(LOG_TARGET)
  docker build -f e2e/Dockerfile -t $(E2E_DOCKER_IMAGE) .

PHONY: test-e2e
test-e2e: ## Run end-to-end tests locally
	@$(LOG_TARGET)
	$(GO) test ./e2e -v

PHONY: test-e2e-docker
test-e2e-docker: build-e2e-image
test-e2e-docker: ## Run end-to-end tests in a Docker container
	@$(LOG_TARGET)
	docker run --rm $(E2E_DOCKER_IMAGE)
