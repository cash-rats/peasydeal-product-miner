# Inngest Endpoint Removal Plan (switch to AMQP)

## Goal
Remove the HTTP Inngest endpoint and its server wiring since the system now uses AMQP, **and remove the `server` service from `docker-compose.yml`**. Keep the worker and AMQP flow intact.

## Scope (what to remove)
- HTTP handler + FX wiring for Inngest.
- Inngest SDK wrapper package used by the handler.
- Inngest-only tests and docs that describe the endpoint.
- Any server-only env vars related to the endpoint (e.g. `INNGEST_*`) if they are no longer used elsewhere.
- The `server` service definition in `docker-compose.yml` (ports, env vars, volumes, command).

## Out of scope
- AMQP worker and enqueue logic.
- Runner / crawl logic used by the worker.
- SQLite/Turso storage used by the worker.

## Planned changes
1. **Delete Inngest HTTP handler and FX module**
   - Remove `internal/app/inngest/handler.go`.
   - Remove `internal/app/inngest/fx/module.go`.

2. **Remove Inngest SDK wrapper package**
   - Remove `internal/pkg/inngest/inngest.go`.

3. **Remove Inngest HTTP tests**
   - Remove `internal/app/inngest/tests/*`.

4. **Unwire from server (if server remains temporarily)**
   - Update `cmd/server/main.go` to drop `inngestfx.Module`.

5. **Cleanup docs and config**
   - Remove or update docs that describe `/api/inngest`.
   - Remove the `server` service block from `docker-compose.yml` (and any server-only env vars like `APP_ADDR`, `APP_PORT`, `INNGEST_*`).
   - Remove `Makefile` targets that exist solely for the endpoint (`start/inngest`).
   - Remove any mention of the endpoint in `docs/server-apis.md`.

6. **Dependency cleanup**
   - Remove Inngest SDK deps from `go.mod`/`go.sum` if no longer referenced.

## Files to edit/remove (expected)
- `internal/app/inngest/handler.go`
- `internal/app/inngest/fx/module.go`
- `internal/app/inngest/tests/`
- `internal/pkg/inngest/inngest.go`
- `cmd/server/main.go`
- `docker-compose.yml`
- `Makefile`
- `docs/server-apis.md`
- `docs/inngest_*` (any that are now obsolete)
- `go.mod`, `go.sum`

## Validation
- `go test ./...` (or at least module-specific tests that remain).
- `go build ./cmd/worker` (worker should still build).
- If `cmd/server` is still present for other routes, `go build ./cmd/server` should still succeed after removing the Inngest module.

## Rollback plan
- Revert the removals and restore the handler + FX module + SDK wrapper, and re-add `INNGEST_*` config.
