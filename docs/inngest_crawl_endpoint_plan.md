# Inngest crawl endpoint plan (Shopee + Taobao)

## Goal

Expose an Inngest HTTP endpoint on the long-lived VPS server so Inngest can dispatch jobs that crawl a Shopee/Taobao product URL using existing code (`internal/source`, `internal/runner`).

## Non-goals

- Reworking crawler logic beyond what’s needed to run via Inngest.
- Introducing new global singletons (DB/Redis/Config/Logger must stay FX-wired).
- Requiring Postgres/Redis to boot the server.

## Proposed API surface

### HTTP endpoint

- `POST /api/inngest`
  - When Inngest is not configured: respond `501 Not Implemented` or `503 Service Unavailable` with `internal/pkg/render.ChiErr`.
  - When configured: delegate to Inngest SDK handler.

### Inngest event(s)

Define one event name that carries the URL:

- Event: `crawler/url.requested`
- Payload:
  - `url` (string, required) — Shopee/Taobao URL (validated with `internal/source.Detect`)
  - `out_dir` (string, optional) — defaults to `CRAWL_OUT_DIR` or `out/`
  - `codex_model` (string, optional) — forwarded to `runner.Options.CodexModel`
  - `skip_git_repo_check` (bool, optional) — forwarded to `runner.Options.SkipGitRepoCheck`
  - `request_id` (string, optional) — for correlation/logging

## Architecture changes (FX + chi)

### 1) Add an Inngest wrapper package

Create `internal/pkg/inngestclient` to keep Inngest SDK wiring isolated and reusable:

- Responsibilities:
  - Read required config/env (via existing `config.Config` if appropriate, or direct env access if config doesn’t yet include these keys).
  - Construct the Inngest client/handler (SDK-specific).
  - Provide “enabled/disabled” state so the HTTP handler can return `501/503` when missing keys.
- Avoid leaking SDK types outside this package (return `http.Handler` or minimal interfaces).

### 2) Implement the Inngest HTTP handler

Fill in `internal/app/inngest/handler.go`:

- `RegisterRoute(r *chi.Mux)` mounts `POST /api/inngest`.
- `Handle(w, r)`:
  - If Inngest is disabled: `render.ChiErr(w, r, http.StatusNotImplemented, "inngest not configured")` (or `503`).
  - Else: forward to the SDK handler (`http.Handler.ServeHTTP`).

Use `*zap.SugaredLogger` for request-level logs (rely on existing middleware for HTTP logs; only add domain logs where useful).

### 3) Wire it via an FX module

Create `internal/app/inngest/fx/module.go`:

- `fx.Options(...)` that provides:
  - the Inngest client/handler from `internal/pkg/inngestclient`
  - the `InngestHandler`
- Register the HTTP handler with routing using `router.AsRoute(inngest.NewInngestHandler)`.

### 4) Enable the module in the VPS entrypoint

Update `cmd/server/main.go` to include `inngestfx.Module` alongside `healthfx.Module`.

Acceptance check: `GET /health` continues to return `200`.

## Job execution design

### Inngest function: crawl URL

Add an Inngest function that runs the existing runner:

- Function name: `crawl_url`
- Trigger: event `crawler/url.requested`
- Steps:
  1. Validate payload (url required; `source.Detect(url)` must succeed).
  2. Build `runner.Options`:
     - `URL`: payload `url`
     - `PromptFile`: optional override (otherwise runner selects by detected source)
     - `OutDir`: payload `out_dir` or `CRAWL_OUT_DIR` default (recommend default `out/`)
     - `CodexCmd`: default `codex` (or `CODEX_CMD` env override)
     - `CodexModel`: payload `codex_model` or `CODEX_MODEL` env default
     - `SkipGitRepoCheck`: payload override defaulting to `true` in containers (optional)
  3. Execute `runner.RunOnce(opts)`.
  4. Emit structured logs including `url`, detected `source`, duration, and output path.
  5. Return a compact result (eg. `{ "out_path": "...", "status": "...", "captured_at": "..." }`).

### Concurrency and timeouts

- Decide an upper bound for concurrent crawls per process (eg. `CRAWL_MAX_CONCURRENCY`).
  - Implementation options:
    - Inngest concurrency controls (preferred if available in the SDK).
    - Local semaphore in the function handler to limit simultaneous `RunOnce` calls.
- Ensure `runner.RunOnce` is run with an explicit context/timeout if the SDK supports it (otherwise wrap the call and enforce timeouts at process level).

## Config / env vars (all optional; server must still start)

- `INNGEST_EVENT_KEY` (or SDK equivalent)
- `INNGEST_SIGNING_KEY` (or SDK equivalent)
- `INNGEST_ENV` (eg. `dev`/`prod`, if required by SDK)
- `CRAWL_OUT_DIR` (default: `out/`)
- `CODEX_CMD` (default: `codex`)
- `CODEX_MODEL` (default: empty -> SDK/runner default behavior)
- `CRAWL_MAX_CONCURRENCY` (default: 1–2 to start)

## Responses and errors

- Use `internal/pkg/render.ChiJSON` and `internal/pkg/render.ChiErr` for the “disabled” case and any direct HTTP errors.
- For SDK-forwarded requests, rely on the SDK response shape (but log enough metadata for debugging).
- Ensure unsupported URLs return a clear error (rooted in `source.Detect`).

## Security

- Require the Inngest signing key verification provided by the SDK (do not accept unsigned requests).
- If SDK verification is not available, add a minimal shared-secret header check for the endpoint until proper verification is implemented.

## Observability

- Correlate logs:
  - Include `request_id` from payload when provided.
  - Log `url`, detected `source`, and the produced output path.
- Consider emitting crawl metrics (counts + latency) if the repo already has metrics patterns; otherwise start with logs only.

## Testing checklist

- Unit tests:
  - `source.Detect` already covers host validation; add tests only if extending source detection.
  - If introducing payload validation helpers, test required/optional behavior.
- Handler tests:
  - When missing Inngest keys: `POST /api/inngest` returns `501/503` and JSON error body.
  - When “enabled” (mock): handler forwards to the underlying `http.Handler`.
- Manual verification:
  - `make start`, then `curl -X POST localhost:$APP_PORT/api/inngest` returns `501/503` without keys.
  - `GET /health` still returns `200`.

## Rollout steps

1. Implement `internal/pkg/inngestclient` and wire it into `internal/app/inngest`.
2. Add `internal/app/inngest/fx/module.go` and enable it in `cmd/server/main.go`.
3. Add the Inngest function for `crawler/url.requested` that calls `runner.RunOnce`.
4. Verify locally with/without Inngest env vars; confirm `/health` unchanged.
5. Document env vars and sample Inngest event payload in `README.md` (optional follow-up).

