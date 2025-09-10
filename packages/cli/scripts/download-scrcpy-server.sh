#!/bin/bash

# Download scrcpy server jar file
# Usage: ./scripts/download-scrcpy-server.sh [version]

set -e

VERSION=${1:-"v3.3.1"}
SERVER_URL="https://github.com/Genymobile/scrcpy/releases/download/${VERSION}/scrcpy-server-${VERSION}"
ASSETS_DIR="assets"
OUTPUT_FILE="${ASSETS_DIR}/scrcpy-server.jar"

# Create assets directory if it doesn't exist
mkdir -p "${ASSETS_DIR}"

echo "Downloading scrcpy-server ${VERSION}..."

# Check if wget or curl is available
if command -v wget >/dev/null 2>&1; then
    wget -O "${OUTPUT_FILE}" "${SERVER_URL}"
elif command -v curl >/dev/null 2>&1; then
    curl -L -o "${OUTPUT_FILE}" "${SERVER_URL}"
else
    echo "Error: Neither wget nor curl is available"
    exit 1
fi

# Verify download
if [ -f "${OUTPUT_FILE}" ]; then
    echo "Successfully downloaded ${OUTPUT_FILE}"
    echo "File size: $(du -h ${OUTPUT_FILE} | cut -f1)"
else
    echo "Error: Failed to download ${OUTPUT_FILE}"
    exit 1
fi