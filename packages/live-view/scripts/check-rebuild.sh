#!/bin/bash

# Check if live-view static files need to be rebuilt
# Returns 0 if rebuild is needed, 1 if files are up to date

BUILD_DIR="static"
SRC_DIR="src"

# Check if build directory exists
if [ ! -d "$BUILD_DIR" ] || [ ! -d "$BUILD_DIR/assets" ]; then
    echo "1"  # Need rebuild
    exit 0
fi

# Get the newest built JS file timestamp
NEWEST_BUILD=$(find "$BUILD_DIR/assets" -name "*.js" -type f -exec stat -f "%m" {} \; 2>/dev/null | sort -rn | head -1)

if [ -z "$NEWEST_BUILD" ]; then
    echo "1"  # Need rebuild
    exit 0
fi

# Check source files
for src in $(find "$SRC_DIR" -name "*.tsx" -o -name "*.ts" -o -name "*.css" -o -name "*.module.css" 2>/dev/null); do
    SRC_TIME=$(stat -f "%m" "$src" 2>/dev/null)
    if [ "$SRC_TIME" -gt "$NEWEST_BUILD" ]; then
        echo "1"  # Need rebuild
        exit 0
    fi
done

# Check config files
for config in index.html vite.config.ts tsconfig.json package.json; do
    if [ -f "$config" ]; then
        CONFIG_TIME=$(stat -f "%m" "$config" 2>/dev/null)
        if [ "$CONFIG_TIME" -gt "$NEWEST_BUILD" ]; then
            echo "1"  # Need rebuild
            exit 0
        fi
    fi
done

echo "0"  # No rebuild needed