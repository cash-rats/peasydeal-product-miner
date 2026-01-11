---
name: adopt
description: Non-destructive adoption of the VPS Go + Uber FX + chi architecture in an existing Go repository. Use when asked to “/adopt”, “initialize this repo to the boilerplate structure”, “introduce FX wiring + chi router”, or “migrate existing routes into the new per-domain long-lived server architecture without deleting old code”.
---

# Adopt (VPS)

## Overview

Add the “long-lived `cmd/server` + FX wiring + chi router + per-domain handlers” structure to an existing Go repo without breaking or deleting the current implementation. Start by adding a parallel `/health` endpoint, then optionally bridge one existing route group behind the new router.

## Workflow

### 0) Hard rules

- Do not delete or rename existing code unless explicitly asked.
- Prefer additive changes and adapters/wrappers.
- Keep baseline `go test ./...` and `go build ./...` working without requiring live Postgres/Redis (treat infra as optional/stubbed).
- Keep the architecture conventions:
  - Long-lived entrypoint at `cmd/server/main.go`
  - FX app boots once and runs an `http.Server` with graceful shutdown
  - Handlers implement `internal/router.Handler` and are registered via `router.AsRoute(...)`

### 1) Preflight discovery (no edits yet)

Confirm:
- `go.mod` exists; record the module path.
- Current server/router locations (search for `http.ListenAndServe`, `chi.NewRouter`, `gin.New`, `echo.New`, `mux.NewRouter`, etc.).
- Whether an existing `cmd/` already exists and if it has an entrypoint (avoid collisions).
- Whether `internal/` already exists (avoid cycles / name conflicts).

If you cannot infer the “first domain to bridge”, ask at most 3 questions:
1) Which route prefix should be migrated first (e.g. `/v1/foo/*`)?
2) Where is the current router/handler that serves that prefix?
3) Should we keep the old server entrypoint working in parallel?

### 2) Add the core scaffold (parallel, minimal)

Add (or adapt to match existing packages):
- `internal/router/core.go`: `Handler` interface + `AsRoute(...)`.
- `internal/router/fx/options.go`: `CoreRouterOptions` providing a `*chi.Mux` that registers all grouped handlers.
- `internal/server/http.go` + `internal/server/fx/options.go`: constructs `http.Server` and wires `fx.Lifecycle` start/stop.
- `internal/pkg/render/render.go`: `ChiJSON` and `ChiErr` helpers (unwrapped JSON).

If the repo doesn’t already provide a zap logger/config via constructors, add minimal providers so FX can build:
- Logger: provide `*zap.Logger` and `*zap.SugaredLogger`.
- Config: Viper-backed typed config with defaults and zero required env vars.

### 3) Prove wiring with `/health`

Add:
- `internal/app/health/...` handler implementing `internal/router.Handler` and returning `{ "ok": true }`.
- `internal/app/health/fx/module.go` registering it via `router.AsRoute(health.NewHandler)`.
- Wire it into the long-lived server aggregation at `cmd/server/main.go`.

### 4) Bridge one existing domain (optional but recommended)

Goal: route requests to a new handler while still using the existing implementation internally.

Add:
- `internal/app/<domain>/legacy.go` implementing `internal/router.Handler`.
  - `RegisterRoute`: register only the target prefix(es) you’re bridging.
  - `Handle`: delegate to the existing handler/router (call into your existing code; do not duplicate business logic).

Add:
- `internal/app/<domain>/fx/module.go` registering the legacy handler via `router.AsRoute(...)`.
- Add that domain module to `cmd/server/main.go`.

### 5) Validation

Run:
- `gofmt` on changed files
- `go mod tidy`
- `go test ./...`
- `go build ./...`

If infra dependencies require live services, gate them behind env vars and return `nil` clients / disabled errors when not configured, so tests/builds don’t fail.

### 6) Handoff notes

Leave the repo in a state where:
- Old entrypoints still work (unless explicitly removed).
- New long-lived server works (`/health` at minimum).
- There’s a clear “bridge handler” pattern to migrate further domains route-by-route.

