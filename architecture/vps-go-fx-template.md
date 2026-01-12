# Go VPS Structure Template Plan (FX + Chi)

This document defines a conservative, reusable boilerplate/template plan for deploying Go services to a VPS (or any long-lived environment) while keeping the same core patterns from this repo: **Uber FX** for dependency injection, **Chi** for HTTP routing, and per-domain handler organization.

The key change vs Vercel: the process is **long-lived**, so we **bootstrap FX once** at startup and run an `http.Server` until shutdown.

## Document Status (Normative When Referenced)

When a human asks to “initialize/adopt/migrate this repo to `architecture/vps-go-fx-template.md`”, treat this document as a **spec**, not an inspiration.

Normative terms:
- **MUST / MUST NOT**: non-negotiable requirements.
- **SHOULD / SHOULD NOT**: strong defaults; deviate only with explicit human instruction.
- **MAY**: optional.

Rules to prevent architecture drift:
- When this document specifies an exact filename, package path, route, or Makefile target name, agents **MUST implement it verbatim**.
- Agents **MUST NOT introduce alternative names/paths/aliases** “for convenience” unless explicitly asked by the human.
- If a deviation seems beneficial, the agent **MUST ask first** and wait for confirmation.

## Acceptance Criteria (Initialization)

An “adoption/initialization” is only complete when all of these are true:
- Entrypoint exists at `cmd/server/main.go` and boots FX **once** (long-lived process).
- HTTP routing uses `chi`, and HTTP handlers implement `internal/router.Handler`.
- At least one route exists: `GET /health` returns `200` and `{ "ok": true }` using `internal/pkg/render.ChiJSON`.
- `make start` exists and runs `go run ./cmd/server` (no alternate targets unless explicitly requested).
- `go test ./...` and `go build ./...` succeed without requiring Postgres/Redis (infra is optional/gated by env vars).

## Deterministic Infra Scaffolding (DB/Redis) — Hard Rules

- Agents **MUST** use the canonical infra packages **exactly** as provided by this template:
  - Postgres: `db/` (SQLX + pgx via `db.NewSQLXPostgresDB`)
  - Redis: `cache/` (go-redis via `cache.NewRedis`)
- Agents **MUST NOT** invent alternative DB/Redis implementations or alternate package paths unless explicitly instructed by a human.
- In a different repo (adoption), if these packages do not exist, agents **MUST** copy `db/` and `cache/` into place (and only adjust import paths/module name as needed), rather than rewriting them from scratch.
- Agents **MUST** implement `config/config.go` to match `architecture/config-go.md` **verbatim** (treat it as a spec).
- Agents **MUST** implement `db/db.go` to match `architecture/db-go.md` **verbatim** (treat it as a spec).
- Agents **MUST** implement `cache/redis.go` to match `architecture/cache-redis-go.md` **verbatim** (treat it as a spec).

Recommended adoption mechanism (deterministic):
- Prefer scaffolding from this repo’s canonical code via `go run github.com/huangc28/vps-go-fx-template/cmd/adopt@latest --dir . --scaffold`
- Then run `go mod tidy` in the target repo to pull required deps.

## 0) Inngest (prebuilt, optional)

The VPS template should include an optional Inngest HTTP endpoint out of the box (same intent as the Vercel variant), implemented as a normal route on the long-lived server:

- Provide a neutral wrapper package: `internal/pkg/inngestclient`.
- Provide an HTTP handler that mounts the Inngest endpoint, e.g. `POST /api/inngest` (or `/inngest`).
- Wire it as a domain-like module: `internal/app/inngest/fx.Module` that contributes its handler via `router.AsRoute(...)`.

The template baseline should still run without Inngest configured; the Inngest handler can respond with a clear `501`/`503` until required env vars are set.

## 1) What Changes vs the Vercel Template

### 1.1 Entry points and lifecycle

Vercel template:
- Entrypoints live at `api/<domain>/core.go`.
- Each request bootstraps an FX app (per-request DI) and executes a handler.
- Routing is mediated by `vercel.json` rewrites.

VPS template:
- Entrypoints live under `cmd/` (for example `cmd/server/main.go`).
- The FX app is created **once** per process and held for the lifetime of the service.
- The HTTP server listens on a port and shuts down gracefully on `SIGINT`/`SIGTERM`.

### 1.2 Folder layout (`lib` -> `internal`)

For VPS services, it’s common to use Go’s `internal/` tree to prevent external imports and keep boundaries clear.

Note: Go convention is `internal/` (singular), not `internals/`. If you strongly prefer `internals/`, it will work, but it won’t get the same tooling/convention benefits.

### 1.3 Routing and “domains”

You can preserve the same domain pattern:
- Keep a router package with a handler interface and grouped registration (via FX).
- Keep handlers under `internal/app/<domain>/...`.
- Register handlers into the router module and mount them under the appropriate path prefix.

### 1.4 Deployment assets

Vercel uses platform-managed build/deploy.
VPS templates typically include one of:
- Dockerfile + `docker-compose.prod.yaml` (recommended for repeatable deploys)

## 2) Template Goal (What we want to extract)

Create a reusable VPS template that mirrors the architecture and ergonomics of this repo:
- FX modules for config/logger/DB/Redis.
- Chi router with handler grouping.
- Minimal example routes (`/health`).
- Optional prebuilt Inngest endpoint.
- Docker-based production build/run workflow.

Non-goals:
- No platform-specific Vercel wiring (`api/<domain>/core.go`, `vercel.json`, `vercel dev`).
- No business-domain code.

## 3) Proposed Template Structure (Conservative)

```
.
├── cmd/
│   ├── adopt/                    # optional: add Codex guidance to an existing repo
│   │   └── main.go               # writes AGENTS + architecture plan (non-destructive)
│   └── server/
│       └── main.go                 # process entrypoint (long-lived)
├── config/
│   └── config.go                   # viper + typed config + defaults
├── cache/
│   └── redis.go                    # redis init (optional)
├── db/
│   ├── db.go                       # sqlx postgres init + fx lifecycle close hooks
│   └── tx.go                       # sqlx transaction helper (Tx wrapper)
├── internal/
│   ├── app/
│   │   ├── fx/
│   │   │   └── core.go             # CoreAppOptions (config/logger/db/redis)
│   │   ├── health/
│   │   │   └── handler.go          # example handler implementing router.Handler
│   │   └── <domain>/
│   │       └── ...                 # domain-specific handlers/services
│   ├── logs/
│   │   └── logs.go                 # logger constructor
│   ├── pkg/
│   │   └── render/                 # ChiJSON / ChiErr response helpers
│   ├── interfaces/
│   │   └── README.md               # cross-domain interfaces (avoid cycles)
│   ├── router/
│   │   ├── core.go                 # router.Handler + registration loop
│   │   └── fx/
│   │       └── options.go          # CoreRouterOptions (chi mux + handler group)
│   └── server/
│       ├── http.go                 # http.Server construction + lifecycle wiring
│       └── fx/
│           └── options.go          # ServerOptions (start/stop hooks)
├── docker-compose.prod.yaml        # production build/run/push
├── Dockerfile
├── Makefile
└── README.md
```

## 4) Dependency Injection + Router Pattern (VPS)

Keep the same conceptual split:

### 4.0 Domain-owned modules (recommended)

Each domain owns its own FX wiring and dependencies, and exposes a single module that `cmd/server/main.go` aggregates. This keeps the entrypoint minimal while preserving clear ownership boundaries.

- Each domain exports `internal/app/<domain>/fx.Module` as an `fx.Option` (typically `fx.Options(...)`).
- The domain module:
  - Provides domain services/repos/controllers via `fx.Provide(...)`.
  - Registers HTTP handlers via `router.AsRoute(New<Handler>)` so the generic router module can collect them (FX group injection).
- Cross-domain dependencies must be expressed via interfaces in `internal/interfaces/<area>` packages to avoid cyclic imports.

### 4.1 Core app module (infra)
- `config.NewViper` and `config.NewConfig`
- `internal/logs.NewLogger` and `internal/logs.NewSugaredLogger`
- `db.NewSQLXPostgresDB` (optional, enabled when `DB_HOST` + `DB_NAME` are present)
- `cache.NewRedis` (optional, enabled when `REDIS_HOST` is present)

### 4.2 Router module
- `internal/router.Handler` interface:
  - `RegisterRoute(r *chi.Mux)`
  - `Handle(w http.ResponseWriter, r *http.Request)`
- `internal/router.AsRoute(...)` to add handlers to an FX group (e.g. `group:"handlers"`).
- `internal/router/fx.CoreRouterOptions` builds a `*chi.Mux`, attaches middleware, and calls `RegisterRoute` on all grouped handlers.

### 4.3 HTTP server module
VPS adds an explicit server module that:
- Builds an `http.Server` with sensible timeouts.
- Starts listening on `APP_PORT` in an `fx.Lifecycle` `OnStart` hook.
- Shuts down gracefully in `OnStop`.

The server module depends on the constructed `*chi.Mux` and config/logger.

## 5) Makefile + Production Workflow

Proposed Makefile targets:
- `make start` runs `go run ./cmd/server`.
- `make build/prod`, `make start/prod`, `make push/prod` use `docker-compose.prod.yaml`.

## 6) Minimal Example Domain (`health`)

Include a minimal health handler to validate wiring:
- Route: `GET /health`
- Response: `200` with `{ "ok": true }` (using `internal/pkg/render.ChiJSON`)

## 7) Env vars (example baseline)

Keep the same config approach (Viper + defaults). Typical vars:
- `APP_NAME` (default: `go-vps-service`)
- `APP_ENV` (default: `development`)
- `APP_PORT` (default: `8080`)
- `LOG_LEVEL` (default: `info`)
- Postgres (optional; disabled unless `DB_HOST` + `DB_NAME` are set):
  - `DB_USER`
  - `DB_PASSWORD`
  - `DB_HOST`
  - `DB_PORT` (default: `5432`)
  - `DB_NAME`
- Redis (optional; disabled unless `REDIS_HOST` is set):
  - `REDIS_USER`
  - `REDIS_PASSWORD`
  - `REDIS_HOST`
  - `REDIS_PORT` (default: `6379`)
  - `REDIS_SCHEME` (default: `redis`, use `rediss` for TLS)

## 8) Logging Convention (Zap Sugared)

- App code (handlers, repos, services) should depend on `*zap.SugaredLogger` rather than `*zap.Logger`.
- Keep `*zap.Logger` available for FX event logging via `fx.WithLogger` (see `cmd/server/main.go`).
