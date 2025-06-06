#!/bin/bash
set -euo pipefail

# Build script for Note-Pulse
# This script generates version information and builds the Go binaries
# Used by both Makefile and Dockerfile to avoid duplication

# Generate version info
REV=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
TAG=$(git describe --tags --dirty --always 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build ldflags
LD_FLAGS="-s -w -X main.version=$TAG -X main.commit=$REV -X main.builtAt=$BUILD_TIME"

# Default values
TARGET="${1:-./cmd/server}"
OUTPUT="${2:-server}"
CGO_ENABLED="${CGO_ENABLED:-0}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-}"

echo "Target OS - $GOOS"
echo "Targte arch - $GOARCH"
echo "Building $TARGET -> $OUTPUT"
echo "Version: $TAG"
echo "Commit: $REV"
echo "Built at: $BUILD_TIME"

# Set CGO and cross-compilation flags if specified
export CGO_ENABLED
if [ -n "$GOOS" ]; then
    export GOOS
fi
if [ -n "$GOARCH" ]; then
    export GOARCH
fi

# Build the binary
go build -trimpath -ldflags "$LD_FLAGS" -o "$OUTPUT" "$TARGET"

echo "Build completed: $OUTPUT"
