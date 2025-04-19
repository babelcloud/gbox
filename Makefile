# Get version from environment variable or git commit
VERSION ?= $(shell git rev-parse --short HEAD)

# Distribution directory
DIST_DIR := dist
DIST_PACKAGES := $(DIST_DIR)/gbox-darwin-amd64-$(VERSION).tar.gz \
                 $(DIST_DIR)/gbox-darwin-arm64-$(VERSION).tar.gz \
                 $(DIST_DIR)/gbox-linux-amd64-$(VERSION).tar.gz \
                 $(DIST_DIR)/gbox-linux-arm64-$(VERSION).tar.gz

# Function to get git commit hash for a path
define get_git_hash
$(shell git log --pretty=tformat:"%h" -n1 -- $(1))
endef

# Image tags
API_SERVER_TAG := $(call get_git_hash,packages/api-server)
MCP_SERVER_TAG := $(call get_git_hash,packages/mcp-server)
PY_IMG_TAG := $(call get_git_hash,images/python)
TS_IMG_TAG := $(call get_git_hash,images/typescript)

# Function to write env var to file (usage: $(call write_env,FILE,VAR,VALUE))
define write_env
	echo "$(2)=$(3)" > $(1)/.env
endef

define append_env
	echo "$(2)=$(3)" >> $(1)/.env
endef

# Check and enable pnpm via corepack
.PHONY: check-pnpm
check-pnpm: ## Check and enable pnpm via corepack
	@if ! command -v pnpm &> /dev/null; then \
		echo "Enabling pnpm via corepack..."; \
		corepack enable; \
		corepack prepare pnpm@latest --activate; \
	fi

# Show help
.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Build all components
.PHONY: build
build: check-pnpm ## Build all components
	@echo "Building Go binary for all platforms..."
	@$(MAKE) -C packages/cli binary-all
	# Binaries are kept in packages/cli/build/
	@echo "Build completed"

# Build docker images
.PHONY: build-images
build-images: ## Build all docker images
	@echo "Building all docker images..."
	@make -C images build-all

.PHONY: build-image-%
build-image-%: ## Build specific docker image (e.g., build-image-python)
	@echo "Building docker image $*..."
	@make -C images build-$*

run-container-%: ## Run specific docker image (e.g., run-container-python)
	@echo "Running docker container $*..."
	@make -C images run-$*

# Create package for specific platform and architecture
.PHONY: dist-%
dist-%: ## Create package for specific platform and architecture (e.g., dist-darwin-amd64)
	@PLATFORM_ARCH=$*; \
	PLATFORM_DIR="$(DIST_DIR)/$$PLATFORM_ARCH"; \
	rm -rf $$PLATFORM_DIR; \
	mkdir -p $$PLATFORM_DIR/bin; \
	mkdir -p $$PLATFORM_DIR/manifests; \
	mkdir -p $$PLATFORM_DIR/packages/mcp-server; \
	mkdir -p $$PLATFORM_DIR/packages/cli; \
	mkdir -p $$PLATFORM_DIR/packages/cli/cmd/script; \
	cp -r manifests/. $$PLATFORM_DIR/manifests/; \
	rsync -a --exclude='node_modules' packages/mcp-server/ $$PLATFORM_DIR/packages/mcp-server/; \
	cp packages/cli/gbox-$$PLATFORM_ARCH $$PLATFORM_DIR/packages/cli/gbox; \
	cp -r packages/cli/cmd/script/. $$PLATFORM_DIR/packages/cli/cmd/script/; \
	cp .env $$PLATFORM_DIR/ 2>/dev/null || true; \
	cp LICENSE README.md $$PLATFORM_DIR/; \
	$(call write_env,$$PLATFORM_DIR/manifests/docker,API_SERVER_IMG_TAG,$(API_SERVER_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,MCP_SERVER_IMG_TAG,$(MCP_SERVER_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,PREFIX,""); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,PY_IMG_TAG,$(PY_IMG_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,TS_IMG_TAG,$(TS_IMG_TAG)); \
	if [ -f "packages/cli/gbox-$$PLATFORM_ARCH" ]; then \
		ln -sf ../packages/cli/gbox $$PLATFORM_DIR/bin/gbox; \
		cp bin/* $$PLATFORM_DIR/bin/ 2>/dev/null || true; \
		(cd $$PLATFORM_DIR && tar -czf ../gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz .env *); \
		(cd $(DIST_DIR) && sha256sum gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz > gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz.sha256); \
		echo "Package created: $(DIST_DIR)/gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz"; \
	else \
		echo "Error: Binary for $$PLATFORM_ARCH not found"; \
		exit 1; \
	fi

# Create all distribution packages
.PHONY: dist
dist: build ## Create all distribution packages
	@echo "Creating all distribution packages..."
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@for platform_arch in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64; do \
		$(MAKE) dist-$$platform_arch; \
	done
	@echo "All distribution packages created:"
	@ls -1 $(DIST_PACKAGES) 2>/dev/null || echo "No packages were created"

# Build and push docker images
.PHONY: docker-push
docker-push: ## Build and push docker images
	@echo "Building and pushing docker images..."
	@make -C packages/api-server docker-push

# Clean distribution files
.PHONY: clean
clean: ## Clean distribution files
	@echo "Cleaning distribution files..."
	@rm -rf $(DIST_DIR)

api-dev: ## Start api server
	@echo "Starting api server..."
	@make -C packages/api-server dev

api: ## Start api server with docker compose
	@cd manifests/docker && docker compose up --build

mcp-dev: ## Start mcp server
	@echo "Starting mcp server..."
	@cd packages/mcp-server && pnpm dev

mcp-inspect: ## Start mcp server
	@echo "Starting mcp server..."
	@cd packages/mcp-server && pnpm inspect

mcp: build ## Start mcp server with distribution package
	@echo "Starting mcp server with distribution package..."
	@cd packages/mcp-server && pnpm inspect:dist

e2e: ## Run e2e tests
	@echo "Running e2e tests..."
	@make -C packages/cli e2e

# Default target
.DEFAULT_GOAL := help 