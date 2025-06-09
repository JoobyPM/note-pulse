#!/bin/bash
set -euo pipefail

# Build script for Note-Pulse
# Generates version information and builds Go binaries
# Used by both Makefile and Dockerfile

# Generate version info
REV=$(git rev-parse --short HEAD 2>/dev/null || echo "none")

# If VERSION is provided (e.g. from Docker ARG) use it, otherwise git-describe
TAG=${VERSION:-$(git describe --tags --dirty --always 2>/dev/null || echo "dev")}

BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build ldflags
LD_FLAGS="-s -w -X main.version=$TAG -X main.commit=$REV -X main.builtAt=$BUILD_TIME"

# Default values
TARGET="${1:-./cmd/server}"
OUTPUT="${2:-server}"
CGO_ENABLED="${CGO_ENABLED:-0}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-}"

echo "Target OS      : $GOOS"
echo "Target arch    : ${GOARCH:-native}"
echo "Building       : $TARGET → $OUTPUT"
echo "Version / tag  : $TAG"
echo "Commit         : $REV"
echo "Built at       : $BUILD_TIME"

export CGO_ENABLED GOOS GOARCH

go build -trimpath -ldflags "$LD_FLAGS" -o "$OUTPUT" "$TARGET"

echo "✓Build completed: $OUTPUT"
