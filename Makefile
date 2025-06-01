# Makefile - build, lint for Note-Pulse
# ───────── meta ─────────
REV         := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
TAG         := $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
BUILD_TIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LD_FLAGS    := -s -w \
	-X main.version=$(TAG) \
	-X main.commit=$(REV) \
	-X main.builtAt=$(BUILD_TIME)

GO_SRCS := $(shell find cmd internal -name '*.go') go.mod go.sum

.PHONY: all build test run vet lint check format swagger install-tools

all: build

## ---------- build / lint / test --------------------------------------
build: bin/server

bin/server: $(GO_SRCS)
	go build -trimpath -ldflags "$(LD_FLAGS)" -o $@ ./cmd/server

test:                  ## unit tests
	go test ./...

run:
	go run ./cmd/server

vet:
	go vet ./...

lint:
	@hash golangci-lint 2>/dev/null || { echo "golangci-lint missing"; exit 1; }
	golangci-lint run ./...

format:
	@echo "Formatting code..."
	go fmt ./...
	@hash goimports 2>/dev/null || \
	  (echo "goimports not installed; skipping" && exit 0)
	@echo "Running goimports..."
	goimports -w .

install-tools:         ## install required tools (golangci-lint)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

check: install-tools format vet lint test build

## ---------- swagger spec ---------------------------------------------
swagger:               ## fresh OpenAPI JSON/YAML
	go run github.com/swaggo/swag/v2/cmd/swag@latest \
		init -g ./cmd/server/main.go --parseDependency --v3.1 --parseInternal --output ./docs/openapi
