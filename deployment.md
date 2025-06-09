# Note‑Pulse — Deployment Guide

> **Scope** This document explains how to build, configure, run and observe
> _Note‑Pulse_ across local development, CI and production. It complements
> **requirements.md** and focuses on operational aspects: container images,
> Compose stacks, Make targets, health‑checks, secrets, monitoring and
> load‑testing.

## 1 Container Image

| Stage       | Base                                                                                        | Purpose                                  |
| ----------- | ------------------------------------------------------------------------------------------- | ---------------------------------------- |
| **builder** | `ghcr.io/joobypm/note-pulse-builder:latest` (Alpine w/ Go toolchain + protobuf, swag, etc.) | compiles binaries and generates Swagger. |
| **runtime** | `gcr.io/distroless/static-debian12:nonroot`                                                 | ultra‑small, no shell, non‑root UID/GID. |

Build flow (see `Dockerfile`):

1. Dependencies restored (`go mod download`).
2. Swagger spec refreshed (`swag init …`).
3. Shared script `./scripts/build.sh` compiles:

   - `./cmd/server   → main`
   - `./cmd/ping     → ping` (lightweight health probe)
4. Final stage copies the two binaries, exposes **8080** and defines
   `ENTRYPOINT ["./main"]`.

> **Version metadata** `scripts/build.sh` injects `main.version`, `main.commit`
> and `main.builtAt` via `-ldflags`. These surface in `/healthz` and Prometheus
> metrics.

## 2 Compose Stacks

> All Compose files share the same service names so they can be combined with
> **`-f`** overrides. Use
>
> ```bash
> docker compose \
>   -f docker-compose.yml \            # base
>   -f docker-compose.rs.yml \         # replica‑set Mongo (optional)
>   -f loadtest/docker-compose.loadtest.yml  # load‑testing (optional)
>   up -d --build
> ```

### 2.1 `docker-compose.yml` — Development default

| Service    | Image                     | Notes                                                                                                                         |
| ---------- | ------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **mongo**  | `mongo:8.0`               | Auth enabled (root user/password from `.env`). Local port **27017** bound. Health‑check: `db.adminCommand('ping')`.           |
| **server** | `notepulse-server:latest` | Built from workspace (`context: .`). Auto‑rebuild on code change (`docker compose up -d --build`). Health‑check: `CMD /ping`. |

`volumes.mongo-data` persists dev data between restarts.

### 2.2 `docker-compose.rs.yml` — Replica‑Set variant

- Converts Mongo to **single‑node RS** (`mongod --replSet rs0`).
- Drops auth - good for local clustering tests.
- Adds an init side‑car **mongo-rs-init** that waits for Mongo then issues
  `rs.initiate(...)`.
- Server is pointed to `MONGO_URI=mongodb://mongo:27017/?replicaSet=rs0`.

### 2.3 `docker-compose.loadtest.yml` — k6 benchmark stack

Activated with Compose **profile `loadtest`**. Changes:

- Mongo runs **in‑memory** (tmpfs) with 1 vCPU / 1 GiB cap.
- Server down‑scales logging, lifts sign‑in rate‑limit and reduces memory.
- Adds **k6** runner container which executes scripts from
  `loadtest/scripts/*.js` and stores results under `loadtest/reports/`.

Run end‑to‑end benchmark via Make:

```bash
make e2e-bench         # starts stack, runs k6, renders markdown report, cleans up
```

### 2.4 `docker-compose.ci.yml` — CI/lightweight

Used in GitHub Actions (or other CI) where persistence and ports are not
required. Mongo data lives in tmpfs, all ports closed. Server health‑check
identical to base file.

## 3 Makefile Targets

| Target              | Description                                                                             |
| ------------------- | --------------------------------------------------------------------------------------- |
| `build`             | Compile **bin/server** with embedded version info.                                      |
| `test`              | Unit tests (`go test ./…`).                                                             |
| `lint`              | `golangci-lint run`. │ Requires toolchain (install via `make install-tools`).           |
| `swagger`           | Refresh OpenAPI JSON/YAML under `docs/openapi`.                                         |
| `dev`               | Generate random secrets (`scripts/gen-dev-env.sh`) then `docker compose up -d --build`. |
| `check`             | Full offline gate: tidy, swagger, fmt, vet, lint, unit tests, e2e compilation.          |
| `e2e` / `e2e-bench` | Run Go E2E tests or full k6 benchmark stack.                                            |

## 4 Environment & Secrets

All configuration is surfaced via **env vars** (12‑factor). For local dev `.env`
is auto‑generated; in CI/Prod inject via your orchestrator.

| Variable                | Default  | Purpose                                                                         |
| ----------------------- | -------- | ------------------------------------------------------------------------------- |
| `APP_PORT`              | 8080     | Server listen port. Health probe uses same port.                                |
| `MONGO_URI`             | _varies_ | Mongo connection string, MUST include credentials when `mongo` is auth‑enabled. |
| `JWT_SECRET`            | random   | 32‑byte base64 for HS256 tokens.                                                |
| `ACCESS_TOKEN_MINUTES`  | 15       | Access token TTL.                                                               |
| `REFRESH_TOKEN_DAYS`    | 30       | Refresh token TTL.                                                              |
| `ROUTE_METRICS_ENABLED` | `true`   | Export Prometheus metrics under `/metrics`.                                     |
| `PPROF_ENABLED`         | `false`  | Enable /debug/pprof.                                                            |
| `PYROSCOPE_ENABLED`     | `false`  | Continuous profiling (see §6).                                                  |

See `scripts/gen-dev-env.sh` for complete reference.

## 5 Health & Readiness

- **`/healthz`** (GET) — returns `200 {"status":"ok"}` when server + Mongo
  reachable.

- **`/ping`** side‑car binary (compiled into image) performs

  1. `GET http://localhost:${APP_PORT}/healthz`
  2. Decodes JSON and exits with status codes **2‑5** on failure variants. Used
     by Docker & orchestrators as liveness / readiness probe.

## 6 Observability Stack

Compose file `monitoring/docker-compose.monitoring.yml` spins up:

| Service        | Port | Notes                                                                              |
| -------------- | ---- | ---------------------------------------------------------------------------------- |
| **Prometheus** | 9090 | Scrapes `/metrics` every 10 s (`prometheus.yml`).                                  |
| **Grafana**    | 3000 | Pre‑provisioned Prometheus + Pyroscope data‑sources. Default creds _admin/admin_.  |
| **Pyroscope**  | 4040 | Continuous CPU/alloc profiling (integrated via `github.com/grafana/pyroscope‑go`). |

Enable server‑side export by setting `PYROSCOPE_ENABLED=true`.

## 7 Load‑Testing Workflow (k6)

1. Build benchmark stack:

   ```bash
   make e2e-bench         # uses .env.bench to override limits
   ```
2. k6 summary JSON is piped to `cmd/k6report`, rendering Git‑friendly Markdown
   diffable reports (see `loadtest/reports/*.report.md`).
3. Baseline deltas computed against the previous committed report (via
   `git ls-files`).

## 8 Production Guidelines

- **Distroless, non‑root** image already hardened.
- Always pin an explicit tag or digest (e.g.
  `notepulse-server:v1.3.2@sha256:…`).
- Provide **external MongoDB** (recommended: 3‑member replica set with auth &
  TLS). Set `MONGO_URI` accordingly.
- Configure `JWT_SECRET`, `LOG_LEVEL`, `SIGNIN_RATE_PER_MIN` via
  secrets/config‑maps.
- Enable TLS termination at ingress (server listens on plain HTTP inside
  container).
- Suggested probes:

  - _liveness_ — `/ping` every 30 s, timeout 5 s, failure threshold 3.
  - _readiness_ — `/healthz` every 10 s.
- Horizontal scaling: server is **stateless**; WebSocket sessions use in‑proc
  pub‑sub, so sticky sessions are recommended if multiple instances are behind
  the same load‑balancer.

## 9 Cheat‑Sheet

```bash
# 🔧 Build image locally
make build && docker build -t notepulse-server:local .

# 🚀 Dev stack with fresh secrets
make dev && docker compose logs -f server

# ✅ Fast CI check (lint + tests + build)
make check-offline

# 🏋️‍♂️  Benchmark isolated endpoints at 100 RPS
make e2e-bench

# 📈 Spin up monitoring stack (Prometheus + Grafana)
docker compose -f docker-compose.yml \
              -f monitoring/docker-compose.monitoring.yml up -d prometheus grafana pyroscope
```

### Appendix A Directory Layout (ops‑relevant)

```
├── Dockerfile                # multi‑stage image
├── docker-compose.yml        # dev stack
├── docker-compose.rs.yml     # replica set override
├── docker-compose.ci.yml     # CI stack
├── loadtest/
│   ├── docker-compose.loadtest.yml
│   ├── scripts/*.js          # k6 scenarios
│   └── reports/*.md/JSON     # generated summaries
├── monitoring/
│   ├── docker-compose.monitoring.yml
│   ├── prometheus/
│   └── grafana/
├── cmd/
│   ├── server/               # main binary
│   ├── ping/                 # health probe helper
│   └── k6report/             # md renderer
├── scripts/
│   ├── build.sh              # shared build
│   └── gen-dev-env.sh        # secrets helper
└── Makefile
```
