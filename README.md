# Shopee Crawler (Dev + Deploy)

This folder is a self-contained, deployable crawler bundle.

## Quickstart (development)

1) Start Chrome with a dedicated profile + DevTools port:

```bash
./scripts/start_host_chrome.sh
```

2) Verify Chrome DevTools is reachable:

```bash
./scripts/doctor.sh
```

3) Run one URL (fast local loop):

```bash
./scripts/run_once.sh "https://shopee.tw/..."
```

Outputs land in `./out/`.

Requires Go 1.22+ installed. Local runs use your host `codex` configuration in `~/.codex/config.toml` (ensure your MCP server points at `http://127.0.0.1:9222`).

## Config files

- Prompt template: `config/prompt.product.txt` (uses `{{URL}}`)
- Output contract: `config/schema.product.json`
- URL list (optional): `config/urls.txt`

## Docker (parity check)

`docker-compose.yml` is a template for running the same runner in Docker while controlling the host Chrome via `host.docker.internal:9222`.
