# Shopee Crawler (Dev + Deploy)

This repo is a self-contained, deployable crawler bundle.

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

`docker-compose.yml` runs the same runner in Docker while controlling the host Chrome via `host.docker.internal:9222`.

```bash
cp .env.example .env
make docker-once TARGET_URL="https://shopee.tw/..."
```
