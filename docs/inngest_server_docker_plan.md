# Inngest Server in Docker (cmd/server) — Plan

## Goal

Run the long-lived HTTP server (`cmd/server/main.go`) inside Docker so it can:

- receive Inngest requests on `POST/PUT/GET /api/inngest`
- run crawl jobs via `internal/app/inngest/crawl/crawl.go` using `codex` or `gemini`
- let users start everything with: edit `.env` → `docker compose up`

## Current Setup (What Exists)

- `cmd/server/main.go` already boots an FX app and includes the `inngest` domain module.
- `internal/app/inngest/fx/module.go` registers the `crawl-url` Inngest function and wires an HTTP handler at `/api/inngest`.
- `internal/app/inngest/crawl/crawl.go` calls `runner.RunOnce(...)` and supports `CRAWL_TOOL=codex|gemini`.
- Docker today is “devtool-in-Docker” only:
  - `Dockerfile` builds `cmd/devtool` and installs Codex/Gemini CLIs.
  - `docker-compose.yml` runs the one-shot `runner` service (exits after a single URL).

## What’s Missing (Gaps to Close)

### 1) No Docker service for the long-lived server

- `docker-compose.yml` has no `server` service, no port mapping, and no long-lived command.
- `Dockerfile` does not build a `cmd/server` binary, so there’s nothing to run in-container.

### 2) Server container likely can’t reach host Chrome DevTools

- `internal/app/inngest/crawl/crawl.go` checks DevTools reachability using `chromedevtools.DefaultHost` which is hardcoded to `127.0.0.1`.
- In Docker Desktop, host Chrome is typically reachable via `host.docker.internal:9222`, not `127.0.0.1:9222` from inside the container.

### 3) Inngest “optional” behavior isn’t clearly gated

- The architecture guidance expects the Inngest endpoint to return `501/503` when missing required keys.
- Today, `internal/pkg/inngest/NewInngestClient` is always constructed and the handler always serves `h.client.Serve()`, even when config is empty (behavior depends on inngestgo internals and may be confusing).

### 4) `.env` and docs don’t cover the “server-in-docker” workflow

- `.env.example` is oriented toward running `make start` on host and using the Docker runner.
- There’s no documented “edit `.env` then `docker compose up`” for the long-lived server.

## Plan (Implementation Steps)

### Step 1 — Add a `server` image target

- Update `Dockerfile` (or add a second Dockerfile) to build **both**:
  - `/app/devtool` from `./cmd/devtool`
  - `/app/server` from `./cmd/server`
- Keep Node + `codex`/`gemini` installed in the final image so the server can execute crawl jobs.
- Ensure the final container has:
  - `/app/config` mounted read-only (prompt templates + schema)
  - writable `/out` for crawl outputs
  - persistent `/codex` + `/gemini` homes for auth + MCP config

### Step 2 — Add `server` service in `docker-compose.yml`

- Add a `server` service that:
  - builds the image
  - runs `/app/server`
  - exposes `APP_PORT` (default `3012`) to the host
  - mounts the same volumes as `runner` (`./out`, `./config`, `./codex`, `./gemini`)
  - sets `APP_ADDR=0.0.0.0` (container-friendly)

### Step 3 — Make DevTools host configurable for container runs

- Without introducing new global singletons, adjust the DevTools reachability check to support Docker Desktop:
  - prefer `CHROME_DEBUG_HOST` (env) when set
  - otherwise default to `127.0.0.1`
- Use that host when calling `chromedevtools.VersionURL(host, cfg.Chrome.DebugPort)`.

### Step 4 — Gate Inngest handler when not configured

- If `INNGEST_APP_ID` or `INNGEST_SIGNING_KEY` is missing (and `INNGEST_DEV` isn’t explicitly enabled), respond with a clear:
  - `501` (“Inngest not configured”) or
  - `503` (“Temporarily unavailable”) per chosen semantics
- Also make `RegisterRoute` respect `INNGEST_SERVE_PATH` when set; otherwise default to `/api/inngest`.

### Step 5 — Make “docker compose up” the primary run path

- Update `.env.example` and `README.md` with:
  - required/optional vars for the server container
  - a minimal quickstart:
    - start Chrome on host (`make dev-chrome`)
    - ensure Codex/Gemini auth exists under `./codex` / `./gemini`
    - `docker compose up --build`
    - verify `GET /health`
    - run Inngest dev (`make start/inngest`) pointing at the exposed server URL

### Step 6 — Verify end-to-end

- `curl http://127.0.0.1:${APP_PORT:-3012}/health`
- Start Inngest dev server and send a `crawler/url.requested` event; confirm:
  - job runs
  - output JSON appears under `out/`
  - logs show tool selection (`codex`/`gemini`)
