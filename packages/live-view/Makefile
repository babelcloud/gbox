.PHONY: build force-build clean dev install

# Default target
all: build

# Install dependencies
install:
	@echo "Installing dependencies..."
	@npm install

# Build static files (checks if rebuild is needed automatically)
build:
	@./scripts/build.sh

# Force rebuild (ignore timestamp checks)
force-build:
	@echo "🔨 Force rebuilding live-view static files..."
	@[ -d "node_modules" ] || npm install
	@npm run build:static && echo "✅ Live-view static files rebuilt successfully" || (echo "❌ Failed to rebuild live-view static files"; exit 1)

# Development mode with hot reload
dev:
	@echo "Starting development server..."
	@npm run dev

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf dist

# Help
help:
	@echo "Available targets:"
	@echo "  install  - Install npm dependencies"
	@echo "  build    - Build static files (only if source changed)"
	@echo "  dev      - Start development server with hot reload"
	@echo "  clean    - Remove build artifacts"
	@echo "  help     - Show this help message"