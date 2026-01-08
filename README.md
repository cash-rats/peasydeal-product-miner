# Shopee Crawler (Dev + Deploy)

This repo is a self-contained, deployable crawler bundle.

## Chrome DevTools MCP (Codex ↔ Chrome)

Codex doesn’t talk to Chrome directly. It talks to an **MCP server** over stdio, and that MCP server talks to a **running Chrome** over the DevTools debug port (`9222`).

### 1) Start Chrome with a dedicated “MCP profile” + debug port

Chrome 136+ requires a **non-default** `--user-data-dir` for remote debugging to work reliably.

```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 \
  --user-data-dir=/tmp/chrome-mcp-profile
```

Verify on the host:
- `http://127.0.0.1:9222/json/version`

Use `make dev-chrome`  directive to launch chrome for codex to perform crawling via chrome MCP server.

### 2) Add Chrome DevTools MCP to Codex (connect to that Chrome)

This writes an MCP entry into your Codex config (typically `~/.codex/config.toml` on the host):

```bash
codex mcp add chrome-devtools-mcp -- \
  npx -y chrome-devtools-mcp@latest --browser-url=http://127.0.0.1:9222
```

If you’re running Codex **inside Docker**, the DevTools URL must point at the host:
- Docker Desktop: `http://host.docker.internal:9222`

## Quickstart (development)

1) Start Chrome with a dedicated profile + DevTools port:

```bash
make dev-chrome
```

2) Verify Chrome DevTools is reachable:

```bash
make dev-doctor
```

3) Run one URL (fast local loop):

```bash
make dev-once URL="https://shopee.tw/..."
```

Outputs land in `./out/`.

Requires Go 1.22+ installed. Local runs use your host `codex` configuration in `~/.codex/config.toml` (ensure your MCP server points at `http://127.0.0.1:9222`).

## Env vars

- `CHROME_DEBUG_PORT` (default `9222`): DevTools port used by `cmd/devtool` commands
- `CHROME_PROFILE_DIR` (default `$HOME/chrome-mcp-profiles/shopee`): dedicated Chrome profile directory for crawling
- `CODEX_CMD` (default `codex`): Codex CLI command used by the runner
- `TARGET_URL` (Docker): URL used by `docker compose` (or `make docker-once`)

## Config files

- Prompt template: `config/prompt.product.txt` (uses `{{URL}}`)
- Output contract: `config/schema.product.json`
- URL list (optional): `config/urls.txt`

## Docker (parity check)

`docker-compose.yml` assumes the runner container can reach host Chrome at `http://host.docker.internal:9222` (Docker Desktop).

```bash
cp .env.example .env
make docker-once TARGET_URL="https://shopee.tw/..."
```

How Docker talks to host Chrome:
- The runner image includes **Node.js + Codex CLI** (see `Dockerfile`), and runs an MCP server via `npx chrome-devtools-mcp@latest`.
- The container’s Codex config is persisted at `./codex/.codex/config.toml` (mounted as `HOME=/codex`), and points the MCP server at the host’s DevTools endpoint (`host.docker.internal:9222`, or an equivalent resolved IP).

## Codex auth in Docker

The container uses `HOME=/codex` and mounts `./codex` to persist Codex auth/config across runs.

```bash
make docker-shell
```

Then run the Codex auth/login command you normally use; the resulting files should land under `./codex/.codex/`.

If you see `Not inside a trusted directory and --skip-git-repo-check was not specified.`, keep `CODEX_SKIP_GIT_REPO_CHECK=1` in `.env`.

## Engineering TODOs

- [ ] Organize it with viper + cobra + fx
- [ ] Determine url whether it's taobao or shopee 執行 corresponding chrome mcp crawler context.
