# AGENTS.md (Codex)

Read `architecture/vps-go-fx-template.md`, `architecture/config-go.md`, `architecture/db-go.md`, and `architecture/cache-redis-go.md` before making structural changes or doing an adoption/migration.

## Architecture (hard rules)
- This repo uses Uber FX for dependency injection. No global singletons for DB/Redis/Config/Logger.
- VPS entrypoint is long-lived: `cmd/server/main.go` bootstraps FX once and runs the HTTP server until shutdown.
- HTTP routing uses `chi`. Handlers must implement `internal/router.Handler`:
  - `RegisterRoute(r *chi.Mux)`
  - `Handle(w http.ResponseWriter, r *http.Request)`
- New HTTP endpoints:
  - Create handler(s) in `internal/app/<domain>/...`
  - Register them via `router.AsRoute(<constructor>)` inside `internal/app/<domain>/fx/module.go`
  - Add the domain module to `cmd/server/main.go`
- Avoid cyclic imports between domains:
  - Put cross-domain interfaces in `internal/interfaces/<area>` packages.
  - Depend on interfaces, not concrete implementations; wire concrete implementations via FX in the caller’s module.

## Infra conventions
- Config is via Viper: `config.NewViper` and `config.NewConfig` (defaults; zero required env vars).
  - `config/config.go` structure and viper defaults **MUST** match `architecture/config-go.md` (treat it as a spec; do not diverge).
- Postgres uses SQLX + pgx via `db.NewSQLXPostgresDB` and must be closed via `fx.Lifecycle` hooks.
  - Enabled when `DB_HOST` + `DB_NAME` are set; otherwise wiring must not block startup.
  - `db/db.go` implementation **MUST** match `architecture/db-go.md` (treat it as a spec; do not diverge).
  - Use `db.Tx` (`db/tx.go`) as the standard transaction wrapper where appropriate.
- Redis uses go-redis via `cache.NewRedis` and must be closed via `fx.Lifecycle` hooks.
  - Enabled when `REDIS_HOST` is set; otherwise wiring must not block startup.
  - `cache/redis.go` implementation **MUST** match `architecture/cache-redis-go.md` (treat it as a spec; do not diverge).
- Deterministic scaffolding: when initializing/adopting this architecture, reuse the template’s `db/` and `cache/` packages as-is (do not invent new DB/Redis implementations or alternate package paths unless explicitly asked).
- Logging uses zap:
  - App logging should use `*zap.SugaredLogger` by default.
  - Keep `*zap.Logger` available for `fx.WithLogger` FX event logs (see `cmd/server/main.go`).
  - FX event logs should be routed through zap (see `fx.WithLogger` in `cmd/server/main.go`).

## Responses
- Use `internal/pkg/render.ChiJSON` and `internal/pkg/render.ChiErr` as the default response helpers (unwrapped JSON).

## Inngest (optional)
- If included, keep the wrapper package generic: `internal/pkg/inngestclient`.
- Implement as a normal route on the long-lived server (e.g. `POST /api/inngest`), gated with `501/503` when missing keys.

## sqlc (optional)
- sqlc config lives at `sqlc.yaml`; queries under `db/query/`.
- Generated code should go into a stable package (documented in `sqlc.yaml`), and must not be edited by hand.
- Schema source lives at `supabase/schema.sql` (Supabase convention), and is pulled/exported manually before running `sqlc generate`.

## Output expectations for scaffolding tasks
When asked to “expose an existing package as a service”:
- Prefer minimal changes to the existing package; wrap it with constructors and FX providers.
- Create one example route and ensure `GET /health` still works.
- Update `README.md` with exact run steps and required env vars (none required; list optional ones).
