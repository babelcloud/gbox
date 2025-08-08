# Docker image names
REGISTRY ?= babelcloud
PLATFORMS ?= linux/amd64,linux/arm64

# Default Python version
PYTHON_VERSION ?= 3.13

# Default Tags for base images
PYTHON_TAG ?= latest
PLAYWRIGHT_TAG ?= latest
TTYD_VERSION ?= 1.7.7 # Default ttyd version

# Default Node.js version for TypeScript
NODE_VERSION ?= 22.18

# ANSI color codes
CYAN := \033[36m
RESET := \033[0m
YELLOW := \033[33m

# Get all directories containing Dockerfile
IMAGES := $(wildcard */Dockerfile)
IMAGES := $(IMAGES:/Dockerfile=)

# Get all environment variables with BUILD_ARG_ prefix and convert them to build args
# Pass common ARGs always
BUILD_ARGS := --build-arg PYTHON_VERSION=$(PYTHON_VERSION) \
              --build-arg PYTHON_TAG=$(PYTHON_TAG) \
              --build-arg PLAYWRIGHT_TAG=$(PLAYWRIGHT_TAG) \
              --build-arg TTYD_VERSION=$(TTYD_VERSION)
BUILD_ARGS += $(foreach v,$(filter BUILD_ARG_%,$(.VARIABLES)),--build-arg $(subst BUILD_ARG_,,$(v)))

# Default command for run target
CMD ?= /bin/bash

# Default container name prefix
CONTAINER_PREFIX ?= gbox-test

.PHONY: build-all clean-all buildx-all help build-% buildx-% clean-% run-% start-% stop-% viewer test-% ttyd-%

# Default target
.DEFAULT_GOAL := help

# Build all images locally
build-all: $(addprefix build-,$(IMAGES)) ## Build all images locally
	@:

# Build all images for multiple architectures
buildx-all: $(addprefix buildx-,$(IMAGES)) ## Build all images for multiple architectures
	@:

# Clean all images
clean-all: $(addprefix clean-,$(IMAGES)) ## Clean all images
	@:

# Stop and remove all containers matching the prefix
stop-all: $(addprefix stop-,$(IMAGES)) ## Stop and remove all containers started by 'make start-*' (for known images)
	@:

# Pattern rule for local builds
build-%: %/Dockerfile ## Build image locally (e.g., build-python)
	docker build \
		$(BUILD_ARGS) \
		-t ${REGISTRY}/gbox-$*:latest \
		-f $< \
		$*

# Pattern rule for multi-arch builds
buildx-%: %/Dockerfile ## Build image for multiple architectures (e.g., buildx-python)
	docker buildx build \
		--platform $(PLATFORMS) \
		--push \
		$(BUILD_ARGS) \
		-t ${REGISTRY}/gbox-$*:latest \
		-f $< \
		$*

# Pattern rule for cleaning specific image
clean-%: ## Clean specific image (e.g., clean-python)
	docker rmi ${REGISTRY}/gbox-$*:latest || true

# Pattern rule for running images interactively (ephemeral)
run-%: build-% ## Run image interactively (removes container on exit)
	@echo "Running image ${REGISTRY}/gbox-$*:latest interactively..."
	docker run --rm -it -P ${REGISTRY}/gbox-$*:latest $(CMD)

# Pattern rule for starting images detached (persistent)
start-%: build-% ## Start image detached via script (e.g., start-python)
	@export REGISTRY=$(REGISTRY) CONTAINER_PREFIX=$(CONTAINER_PREFIX); \
	 ./scripts/start.sh $*

# Pattern rule for stopping and removing detached containers
stop-%: ## Stop and remove container via script (e.g., stop-python)
	@export CONTAINER_PREFIX=$(CONTAINER_PREFIX); \
	 ./scripts/stop.sh $*

# Pattern rule for opening ttyd web terminal
ttyd-%: start-% ## Open ttyd web terminal via script (e.g., ttyd-python)
	@export CONTAINER_PREFIX=$(CONTAINER_PREFIX); \
	 ./scripts/ttyd.sh $*

# Pattern rule for testing images
test-%: start-% ## Test image via script (e.g., test-python)
	@export CONTAINER_PREFIX=$(CONTAINER_PREFIX); \
	 ./scripts/test.sh $*

# Target for viewer convenience script
viewer: start-viewer ## Open viewers for the running viewer container
	@./scripts/viewer.sh "${CONTAINER_PREFIX}-viewer"

# Show help
help: ## Show this help message
	@echo "Available targets:"
	@echo ""
	@echo "General targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' Makefile | grep -v '%' | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "Pattern rules (replace % with image name):"
	@grep -E '^[a-zA-Z0-9_-]+-%:.*?## .*$$' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-25s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "Available images:"
	@for img in $(IMAGES); do \
		printf "  $(CYAN)%s$(RESET)\n" "$$img"; \
	done

# --- Explicit Build Dependencies --- #
# These ensure images are built in the correct order when using build-all
build-playwright: build-python
build-viewer: build-playwright