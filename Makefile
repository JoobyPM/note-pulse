# Makefile - build, lint for Note-Pulse

GO_SRCS := $(shell find cmd internal -name '*.go') go.mod go.sum

.PHONY: all build test run vet lint check format swagger install-tools e2e-check e2e tidy

all: build

## ---------- build / lint / test --------------------------------------
build: bin/server

bin/server: $(GO_SRCS) scripts/build.sh
	@chmod +x scripts/build.sh
	./scripts/build.sh ./cmd/server $@

test:                  ## unit tests
	go test ./...

run:
	go run ./cmd/server

vet:
	go vet ./...

tidy:
	go mod tidy

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

check: tidy swagger install-tools format vet lint test build e2e-check

e2e:
	go test -tags e2e ./test -timeout 2m -v

e2e-check:
	go vet -tags=e2e ./test && golangci-lint run --build-tags=e2e ./test

## ---------- swagger spec ---------------------------------------------
swagger:               ## fresh OpenAPI JSON/YAML
	go run github.com/swaggo/swag/cmd/swag@latest \
		init -g ./docs/swagger.go --parseDependency --parseInternal --output ./docs/openapi
