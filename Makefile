# Makefile - build, lint for Note-Pulse

GO_SRCS := $(shell find cmd internal -name '*.go') go.mod go.sum

# ------------------------------------------------------------
# common compose file list for the benchmark stack
# ------------------------------------------------------------
BENCH_PROJECT ?= np_bench
BENCH_COMPOSE = \
	-f docker-compose.yml \
	-f docker-compose.rs.yml \
	-f loadtest/docker-compose.loadtest.yml

.PHONY: all build test run vet lint check format swagger install-tools e2e-check e2e tidy k6-report e2e-clean e2e-bench k6-report

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

install-tools:         ## install required tools (golangci-lint,swag)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
	go install github.com/swaggo/swag/cmd/swag@v1.16.4
	go install golang.org/x/tools/cmd/goimports@v0.34.0

check: tidy swagger format vet lint test build e2e-check

check-offline: format vet lint test build e2e-check

e2e:
	go test -tags e2e ./test -timeout 2m

e2e-debug:
	go test -tags e2e ./test -timeout 2m -v

e2e-check:
	go vet -tags=e2e ./test && golangci-lint run --build-tags=e2e ./test

# ------------------------------------------------------------
# wipe the benchmark stack completely
# ------------------------------------------------------------
e2e-clean:
	@echo "▶ cleaning benchmark project: $(BENCH_PROJECT)"
	docker compose -p $(BENCH_PROJECT) $(BENCH_COMPOSE) down \
		--remove-orphans \
		--volumes \
		--rmi local

# ------------------------------------------------------------
# build, run k6, collect the markdown report
# ------------------------------------------------------------
e2e-bench: e2e-clean
	@echo "▶ starting benchmark stack"
	@export $$(cat .env.bench | grep -v '^#' | grep -v '^$$' | xargs) && \
	ENV_FILE=.env.bench docker compose -p $(BENCH_PROJECT) $(BENCH_COMPOSE) \
		--env-file .env.bench \
		--profile loadtest \
		up --abort-on-container-exit \
		--build \
		--force-recreate \
		--always-recreate-deps \
		k6
	$(MAKE) k6-report
	$(MAKE) e2e-clean
	@echo "✔ benchmark finished"

# ------------------------------------------------------------
# turn JSON into Markdown
# ------------------------------------------------------------
k6-report:
	@go run ./cmd/k6report < loadtest/reports/user_flow.summary.json \
		> loadtest/reports/user_flow.report.md
		deno fmt loadtest/reports
	@echo "✔ report regenerated"

## ---------- swagger spec ---------------------------------------------
swagger:               ## fresh OpenAPI JSON/YAML
	swag init -g ./docs/swagger.go --parseDependency --parseInternal --output ./docs/openapi

dev: scripts/gen-dev-env.sh ## start development environment with auto-generated secrets
	@./scripts/gen-dev-env.sh
	docker compose up -d --build
	docker compose logs -f server
