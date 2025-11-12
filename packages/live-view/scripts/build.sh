#!/bin/bash

# Build live-view static files if needed

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Check if rebuild is needed
NEED_BUILD=$("$SCRIPT_DIR/check-rebuild.sh")

if [ "$NEED_BUILD" = "1" ]; then
    echo "🔨 Building live-view static files (source files changed)..."
    
    # Install dependencies if needed
    if [ ! -d "node_modules" ]; then
        echo "Installing dependencies..."
        npm install
    fi
    
    # Build static files
    npm run build:static
    
    if [ $? -eq 0 ]; then
        echo "✅ Live-view static files built successfully"
    else
        echo "❌ Failed to build live-view static files"
        exit 1
    fi
else
    echo "✓ Live-view static files are up to date"
fi