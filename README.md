# Shopee Product Crawler (Codex + Chrome DevTools)

This repo crawls Shopee product pages by controlling a **real Chrome** (logged-in session) via **DevTools remote debugging**, using **Codex CLI** + **chrome-devtools-mcp**.

The crawler writes **JSON files** to `out/` that must conform to `config/schema.product.json` (including `status: "ok" | "needs_manual" | "error"`).

## How It Works (1 minute)

- You run **Chrome on your host** with DevTools enabled (`:9222`) and a **dedicated profile directory** (required for Chrome 136+).
- Codex talks to an MCP server (`chrome-devtools-mcp`) over stdio.
- The MCP server talks to Chrome over the DevTools endpoint.

## Quickstart (Local Dev)

### 0) Prerequisites

- Go 1.24+
- Codex CLI installed on your host
- Node.js available (for `npx chrome-devtools-mcp@latest`)
- Google Chrome installed

### 1) Start Chrome (dedicated profile + DevTools port)

```bash
make dev-chrome
```

This launches Chrome with:
- `--remote-debugging-address=$CHROME_DEBUG_BIND_ADDR` (default `127.0.0.1`; use `0.0.0.0` when the crawler runs in Docker)
- `--remote-debugging-host=$CHROME_DEBUG_BIND_ADDR` (compat flag; same value)
- `--remote-debugging-port=9222`
- `--user-data-dir=...` (non-default; required for Chrome 136+)

Verify it works (on the host):
- open `http://127.0.0.1:9222/json/version`

### 2) One-time: connect Codex to that Chrome

```bash
codex mcp add chrome-devtools-mcp -- \
  npx -y chrome-devtools-mcp@latest --browser-url=http://127.0.0.1:9222
```

This updates your host Codex config (typically `~/.codex/config.toml`).

### 2b) One-time: connect Gemini to that Chrome

Gemini CLI reads MCP servers from its settings file (this repo tracks `gemini/.gemini/settings.json`).

To use the repo’s config on the host:

```bash
export GEMINI_CLI_SYSTEM_SETTINGS_PATH="$PWD/gemini/.gemini/settings.json"
export CHROME_DEBUG_HOST=127.0.0.1
export CHROME_DEBUG_PORT=9222
```

Then run `gemini` normally; it will launch `chrome-devtools-mcp` via the configured MCP server.

### 3) Verify the devtools connection

```bash
make dev-doctor
```

### 4) Crawl one product URL

```bash
make dev-once url=https://shopee.tw/...
```

Results land in `out/`.

## What To Edit

- Prompt template: `config/prompt.product.txt` (uses `{{URL}}`, see also `config/prompt.shopee.product.txt` / `config/prompt.taobao.product.txt`)
- Output contract (schema): `config/schema.product.json`
- URL list (optional): `config/urls.txt`

## Environment Variables

- `CHROME_DEBUG_PORT` (default `9222`): DevTools port used by `cmd/devtool`
- `CHROME_PROFILE_DIR` (default `$HOME/chrome-mcp-profiles/shopee`): dedicated Chrome profile directory for crawling
- `CODEX_MODEL` (optional): pass `--model` to `codex exec` (use a faster model to reduce crawl latency)
- `TARGET_URL` (Docker, optional): URL used by `docker compose run runner` when you don't pass `--url` explicitly

## Docker (Deployment Parity Check)

This repo supports a “runner-in-Docker, Chrome-on-host” deployment model.

1) Start host Chrome first:

```bash
make dev-chrome
```

2) Run a single crawl inside Docker:

```bash
cp .env.example .env
make docker-once url=https://shopee.tw/...
```

Notes:
- Docker connects to host Chrome via `http://host.docker.internal:9222` (Docker Desktop).
- The container persists Codex config/auth under `codex/.codex/` by mounting `./codex` as `HOME=/codex`.

### Codex auth in Docker (if needed)

```bash
make docker-login tool=codex
```

This runs Codex login on the host while storing auth/config under `./codex/.codex/` (which is mounted into the container).

For interactive debugging inside the container:

```bash
make docker-shell
```

If you see `Not inside a trusted directory...`, keep `CODEX_SKIP_GIT_REPO_CHECK=1` in `.env`.

## When You See `status: "needs_manual"`

Shopee sometimes shows a login/verify/CAPTCHA wall that is not reliably automatable.

- Fix it by manually completing the verification in the **same Chrome profile** you started with `make dev-chrome`.
- Re-run `make dev-once url=...` (or let the scheduled runner try again later).

## Troubleshooting

- Chrome DevTools not reachable:
  - confirm Chrome was started by `make dev-chrome` (Chrome 136+ requires a non-default `--user-data-dir`)
  - confirm `http://127.0.0.1:9222/json/version` works on the host
  - check port `9222` isn’t already in use
- Docker can’t talk to Chrome:
  - ensure you’re using Docker Desktop and the MCP browser URL is `http://host.docker.internal:9222`

## Safety

Do not expose port `9222` to your LAN/Internet; a DevTools session can fully control the browser.

## VPS HTTP Server (FX + chi)

This repo also includes a long-lived HTTP server (Uber FX + chi) with:
- `GET /health`
- an optional Inngest endpoint at `POST/PUT/GET /api/inngest` (used to receive crawl jobs)

Run:

```bash
make start
```

Run with SQLite enabled (requires `go get modernc.org/sqlite` once):

```bash
TURSO_SQLITE_PATH=./out/dev.sqlite go run -tags=sqlite ./cmd/server
```

Verify:

```bash
curl -sS http://127.0.0.1:8080/health
```

Optional env vars (all have defaults; Postgres/Redis are disabled unless configured):
- `APP_PORT` (default `8080`)
- `LOG_LEVEL` (default `info`)
- Postgres (enabled only when `DB_HOST` + `DB_NAME` are set): `DB_USER`, `DB_PASSWORD`, `DB_HOST`, `DB_PORT`, `DB_NAME`
- Redis (enabled only when `REDIS_HOST` is set): `REDIS_USER`, `REDIS_PASSWORD`, `REDIS_HOST`, `REDIS_PORT`, `REDIS_SCHEME`
- SQLite (enabled only when `TURSO_SQLITE_PATH` or `TURSO_SQLITE_DSN` is set; requires building/running with `-tags=sqlite`): `TURSO_SQLITE_PATH`, `TURSO_SQLITE_DSN`, `TURSO_SQLITE_TOKEN`, `TURSO_SQLITE_DRIVER` (default `sqlite`)

## Docker: Long-Lived Server (Inngest Worker)

This is the “just edit `.env` then `docker compose up`” path for running the Inngest worker server in a container.

1) Start host Chrome first:

```bash
make dev-chrome
```

2) Create `.env`:

```bash
cp .env.example .env
```

3) Bring up the server:

```bash
docker compose up --build server
```

Verify:

```bash
curl -sS http://127.0.0.1:${APP_PORT:-3012}/health
```

Notes:
- The `server` container always binds to `0.0.0.0` internally so the published port works (even if your `.env` uses `APP_ADDR=localhost` for host runs).
- The container expects host Chrome DevTools at `http://${CHROME_DEBUG_HOST:-host.docker.internal}:${CHROME_DEBUG_PORT:-9222}`.
- In Docker, the DevTools hostname is resolved to an IPv4 address before checking `/json/version` (helps avoid IPv6/hostname quirks).
- In Docker, `/app/out` is symlinked to `/out` so relative `out/` writes land in the host-mounted `./out`.
- For Docker, start Chrome with `CHROME_DEBUG_BIND_ADDR=0.0.0.0 make dev-chrome` so the container can reach the DevTools port.
- If you're using `inngest-cli dev` on the host, set `INNGEST_DEV=http://host.docker.internal:8288` so the container can register/sync with the dev server.
- Codex/Gemini auth is persisted via `./codex/.codex` and `./gemini/.gemini` volume mounts.
- Containers use `HOME=/home/app` and symlink `~/.codex` → `/codex/.codex`, `~/.gemini` → `/gemini/.gemini` (see `entrypoint.sh`).
