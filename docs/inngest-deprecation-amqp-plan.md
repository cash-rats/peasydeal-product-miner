# Inngest Deprecation → AMQP (RabbitMQ) — Fix Plan

Date: 2026-01-27

## Recommendation: move ProductDraftStore out of the Inngest domain

**Suggested location:** `internal/app/productdrafts` (or `internal/app/drafts`) with a small `fx` submodule if needed later.

**Why this is a good home:**
- The store is a core persistence component used by both the AMQP worker and any future producers/HTTP APIs; it is not inherently tied to Inngest.
- Keeps domain boundaries clean: `internal/app/inngest` should not be imported by AMQP code if Inngest is being deprecated.
- Avoids future cyclic dependencies as more domains (admin API, dispatcher, etc.) need to write drafts.
- Aligns with repo conventions: per-domain packages live under `internal/app/<domain>` and expose constructors + FX wiring.

If you want the most minimal change, keep the store in place for now and only delete the handler/server pieces. But to fully remove Inngest, the store should move.

---

## Fix Plan

### 1) Relocate ProductDraftStore (decouple from Inngest)
- Create `internal/app/productdrafts/store.go` by moving `internal/app/inngest/dao/store.go`.
- Update package name to `productdrafts`.
- Update imports:
  - `internal/app/amqp/crawlworker/handler.go`
  - `cmd/worker/main.go`
  - Any other references to `internal/app/inngest/dao`.
- Confirm all references compile and tests (if any) are updated to new package path.

### 2) Remove Inngest server wiring (keep a reference example)
- Remove `internal/app/inngest/fx` module from `cmd/server/main.go` so the production server is Inngest-free.
- Add a **non-deployed** reference entrypoint, e.g. `cmd/server-inngest-example/main.go`, that shows how to wire the Inngest module (for future reference only).
- Delete Inngest handler and function registration:
  - `internal/app/inngest/handler.go`
  - `internal/app/inngest/fx/module.go`
  - `internal/app/inngest/crawl/*`
  - `internal/pkg/inngest/*`
  - `internal/app/inngest/tests/*`

### 3) Clean configuration + docs
- Remove Inngest config fields + defaults from `config/config.go`.
- Remove Inngest env vars from:
  - `.env.example`, `.env.prod.example`
  - `docker-compose.yml` (server env block)
  - `Makefile` (start/inngest target)
- Delete or update Inngest-focused docs:
  - `docs/inngest_*` and any Inngest plans that are no longer relevant.
  - Keep RabbitMQ docs as the canonical flow.

### 4) Clean dependencies
- Remove Inngest SDK deps from `go.mod` and `go.sum`.
- Run `go mod tidy` after deletions.

### 5) Validate build/runtime
- `go build ./cmd/server` (server should still boot and serve `/health`).
- `go build ./cmd/worker` (AMQP worker still runs with RabbitMQ + SQLite).

---

## Notes / Constraints

- No AMQP producer work is included (per request). You will handle publishing via cron.
- If you prefer to keep the store where it is temporarily, Step 1 can be skipped; the rest of the plan still removes server/handler.
- Ensure any SQLite schema / migration references to `product_drafts` remain unchanged.
