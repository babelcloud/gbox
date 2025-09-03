# Makefile for gbox CLI

# Common prefix for Go packages
MODULE_PREFIX := github.com/babelcloud/gbox

# Variable definitions
BINARY_NAME := gbox
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT_ID := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DIR := .
MAIN_FILE := main.go

# Default platform (based on current system)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# LDFLAGS for embedding version information
LDFLAGS := -ldflags "-X $(MODULE_PREFIX)/packages/cli/internal/version.Version=$(VERSION) \
                     -X $(MODULE_PREFIX)/packages/cli/internal/version.BuildTime=$(BUILD_TIME) \
                     -X $(MODULE_PREFIX)/packages/cli/internal/version.CommitID=$(COMMIT_ID) \
					 -X $(MODULE_PREFIX)/packages/cli/config.githubClientSecret=$(GBOX_GITHUB_CLIENT_SECRET)"

# Supported platforms
PLATFORMS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64

.PHONY: all clean help binary binary-all install e2e-setup e2e e2e-env-check e2e-cleanup e2e-% e2e-ci e2e-ci-% run test

# Default target
all: help ## Show this help message (default target)

# Help information
help: ## Show this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Run test for a specific command using pattern matching
.PHONY: test-%
test-%: binary ## Run test for a specific command (e.g. make test-create, builds binary first)
	@if [ -f ../../.env ]; then \
		cd e2e && GBOX_API_KEY=$$(grep '^GBOX_API_KEY=' ../../../.env | cut -d'=' -f2) ./run-test.exp --$*; \
	else \
		cd e2e && ./run-test.exp --$*; \
	fi

# Run tests for all commands
test: binary ## Run tests for all commands (builds binary first)
	@if [ -f ../../.env ]; then \
		cd e2e && GBOX_API_KEY=$$(grep '^GBOX_API_KEY=' ../../../.env | cut -d'=' -f2) ./run-test.exp; \
	else \
		cd e2e && ./run-test.exp; \
	fi

# Clean build directory
clean: ## Clean the build directory
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)*
	@echo "Cleaning completed"

# Build binary for a single platform
binary: ## Build binary for the current platform (GOOS/GOARCH)
	@echo "Building $(BINARY_NAME) binary ($(GOOS)/$(GOARCH))..."
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_FILE)
	@echo "Binary built: $(BINARY_NAME)"

# Run the application with project root as environment variable
# Usage: make run box list
.PHONY: run
run: ## Run the application with PROJECT_ROOT environment variable (Usage: make run cmd args...)
	@PROJECT_ROOT=$$(git rev-parse --show-toplevel) \
	CLI_DEV_MODE=true \
	go run main.go $(filter-out $@,$(MAKECMDGOALS))

go-test: ## Run tests
	go test ./... -v

# Build binaries for all supported platforms
binary-all: ## Build binaries for all supported platforms
	@echo "Building binaries for all supported platforms..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d- -f1); \
		arch=$$(echo $$platform | cut -d- -f2); \
		outfile="$(BINARY_NAME)-$$platform"; \
		echo "Building $$outfile..."; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $$outfile $(MAIN_FILE); \
	done
	@echo "All platform binaries have been built"


# Add this to prevent make from trying to build the arguments
%:
	@:
