SHELL:=/bin/bash

DATETIME = $(shell date +"%Y%m%d%H%M%S")

BINARY_NAME := typo
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := bin
GO := go
E2E_DOCKER_IMAGE ?= typo-e2e:local

# Log the running target
LOG_TARGET = echo -e "\033[0;32m==================> Running $@ ============> ... \033[0m"
# Log debugging info

define log
echo -e "\033[36m==================>$1\033[0m"
endef
# Log error info

define errorLog
echo -e "\033[0;31m==================>$1\033[0m"
endef

.PHONY: help
help:
	@echo -e "\033[1;3;34m Typo, Fixed command error 🛠️.\033[0m\n"
	@echo -e "Usage:\n  make \033[36m<Target>\033[0m \033[36m<Option>\033[0m\n\nTargets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
