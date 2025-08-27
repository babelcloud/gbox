# ==============================================================================
# Build Variables
# ==============================================================================
export GBOX_GITHUB_CLIENT_SECRET
MODULE_PREFIX := github.com/babelcloud/gbox

# Check if .git directory exists to determine version from git
ifneq ($(wildcard .git),)
  VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
  COMMIT_ID ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
else
  VERSION ?= dev
  COMMIT_ID ?= unknown
endif

BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

# LDFLAGS for embedding version information. These variables can be overridden from the command line.
LDFLAGS := -ldflags "-s -w -X '$(MODULE_PREFIX)/packages/cli/internal/version.Version=$(VERSION)' \
                     -X '$(MODULE_PREFIX)/packages/cli/internal/version.BuildTime=$(BUILD_TIME)' \
                     -X '$(MODULE_PREFIX)/packages/cli/internal/version.CommitID=$(COMMIT_ID)' \
                     -X '$(MODULE_PREFIX)/packages/cli/config.githubClientSecret=$(GBOX_GITHUB_CLIENT_SECRET)' 
# ==============================================================================


# Distribution directory
DIST_DIR := dist
DIST_PACKAGES := $(DIST_DIR)/gbox-darwin-amd64-$(VERSION).tar.gz \
                 $(DIST_DIR)/gbox-darwin-arm64-$(VERSION).tar.gz \
                 $(DIST_DIR)/gbox-linux-amd64-$(VERSION).tar.gz \
                 $(DIST_DIR)/gbox-linux-arm64-$(VERSION).tar.gz \
                 $(DIST_DIR)/gbox-windows-amd64-$(VERSION).zip \
                 $(DIST_DIR)/gbox-windows-arm64-$(VERSION).zip

# Function to get git commit hash for a path
define get_git_hash
$(shell git log --pretty=tformat:"%h" -n1 -- $(1))
endef

# Image tags
API_SERVER_TAG := $(call get_git_hash,packages/api-server)
MCP_ANDROID_SERVER_TAG := $(call get_git_hash,packages/mcp-android-server)
PY_IMG_TAG := $(call get_git_hash,images/python)
PW_IMG_TAG := $(call get_git_hash,images/playwright)
VNC_IMG_TAG := $(call get_git_hash,images/viewer)
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
	mkdir -p $$PLATFORM_DIR/packages/mcp-android-server; \
	mkdir -p $$PLATFORM_DIR/packages/cli; \
	mkdir -p $$PLATFORM_DIR/packages/cli/cmd/script; \
	cp -r manifests/. $$PLATFORM_DIR/manifests/; \
	rsync -a --exclude='node_modules' packages/mcp-android-server/ $$PLATFORM_DIR/packages/mcp-android-server/; \
	cp packages/cli/gbox-$$PLATFORM_ARCH $$PLATFORM_DIR/packages/cli/gbox; \
	cp -r packages/cli/cmd/script/. $$PLATFORM_DIR/packages/cli/cmd/script/; \
	cp .env $$PLATFORM_DIR/ 2>/dev/null || true; \
	cp LICENSE README.md $$PLATFORM_DIR/; \
	$(call write_env,$$PLATFORM_DIR/manifests/docker,API_SERVER_IMG_TAG,$(API_SERVER_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,MCP_ANDROID_SERVER_IMG_TAG,$(MCP_ANDROID_SERVER_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,PREFIX,""); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,PY_IMG_TAG,$(PY_IMG_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,PW_IMG_TAG,$(PW_IMG_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,VNC_IMG_TAG,$(VNC_IMG_TAG)); \
	$(call append_env,$$PLATFORM_DIR/manifests/docker,TS_IMG_TAG,$(TS_IMG_TAG)); \
	if [ -f "packages/cli/gbox-$$PLATFORM_ARCH" ]; then \
		ln -sf ../packages/cli/gbox $$PLATFORM_DIR/bin/gbox; \
		cp bin/* $$PLATFORM_DIR/bin/ 2>/dev/null || true; \
		case "$$PLATFORM_ARCH" in \
		windows-*) \
			(cd $$PLATFORM_DIR && zip -qr ../gbox-$$PLATFORM_ARCH-$(VERSION).zip .env *); \
			(cd $(DIST_DIR) && sha256sum gbox-$$PLATFORM_ARCH-$(VERSION).zip > gbox-$$PLATFORM_ARCH-$(VERSION).zip.sha256); \
			echo "Package created: $(DIST_DIR)/gbox-$$PLATFORM_ARCH-$(VERSION).zip"; \
			;; \
		*) \
			(cd $$PLATFORM_DIR && tar -czf ../gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz .env * > /dev/null); \
			(cd $(DIST_DIR) && sha256sum gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz > gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz.sha256); \
			echo "Package created: $(DIST_DIR)/gbox-$$PLATFORM_ARCH-$(VERSION).tar.gz"; \
			;; \
		esac; \
	else \
		echo "Error: Binary for $$PLATFORM_ARCH not found"; \
		exit 1; \
	fi

# Brew distribution directory
BREW_DIST_DIR ?= brew

.PHONY: brew-dist
brew-dist: ## Create a distribution for Homebrew
	@echo "Creating Homebrew distribution in $(BREW_DIST_DIR)..."
	@rm -rf $(BREW_DIST_DIR)
	@mkdir -p $(BREW_DIST_DIR)/bin

	@echo "Building gbox binary..."
	@(cd packages/cli && go build $(LDFLAGS) -o $(abspath $(BREW_DIST_DIR))/bin/gbox .)

	@echo "Homebrew distribution is ready at $(BREW_DIST_DIR)"

# Create all distribution packages
.PHONY: dist
dist: build ## Create all distribution packages
	@echo "Creating all distribution packages..."
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@for platform_arch in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64 windows-arm64; do \
		$(MAKE) dist-$$platform_arch; \
	done
	@echo "All distribution packages created:"
	@ls -1 $(DIST_PACKAGES) 2>/dev/null || echo "No packages were created"

.PHONY: dist-source
dist-source: ## Create a source code distribution package
	@echo "Creating source distribution..."
	@git rev-parse HEAD > COMMIT
	@rm -rf $(DIST_DIR)/gbox-$(VERSION)
	@mkdir -p $(DIST_DIR)/gbox-$(VERSION)
	@echo "Copying source files..."
	@git archive HEAD | tar -x -C $(DIST_DIR)/gbox-$(VERSION)
	@cp COMMIT $(DIST_DIR)/gbox-$(VERSION)/COMMIT
	@echo "Creating source archive..."
	@(cd $(DIST_DIR) && tar -czf gbox-v$(VERSION).tar.gz gbox-$(VERSION))
	@rm -rf $(DIST_DIR)/gbox-$(VERSION)
	@rm COMMIT
	@(cd $(DIST_DIR) && sha256sum gbox-v$(VERSION).tar.gz > gbox-v$(VERSION).tar.gz.sha256)
	@echo "Source distribution package created: $(DIST_DIR)/gbox-v$(VERSION).tar.gz"
	@echo "SHA256 checksum created: $(DIST_DIR)/gbox-v$(VERSION).tar.gz.sha256"

# Install for Homebrew
.PHONY: install
install: ## Install for Homebrew
	@$(MAKE) brew-dist BREW_DIST_DIR=$(prefix) VERSION=$(VERSION) COMMIT_ID=$(COMMIT_ID) BUILD_TIME=$(BUILD_TIME)

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

mcp-android-dev: ## Start mcp android server
	@echo "Starting mcp android server..."
	@cd packages/mcp-android-server && pnpm dev

mcp-android-inspect: ## Start mcp android server with distribution package
	@echo "Starting mcp android server with distribution package..."
	@cd packages/mcp-android-server && pnpm inspect:dist

mcp-android: build ## Start mcp android server with distribution package
	@echo "Starting mcp android server with distribution package..."
	@cd packages/mcp-android-server && pnpm inspect:dist

e2e: ## Run e2e tests
	@echo "Running e2e tests..."
	@make -C packages/cli e2e

# Default target
.DEFAULT_GOAL := help