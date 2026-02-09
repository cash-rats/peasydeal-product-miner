# Peasydeal Product Miner — RabbitMQ Worker (Docker Compose)

This repo is focused on running the RabbitMQ crawl worker. Follow the steps below to bring it up on a dev machine.

## Prerequisites

- Docker + Docker Compose
- Google Chrome (for DevTools)

## 1) Start Chrome DevTools on the host

```bash
make dev-chrome
```

If Docker can’t reach Chrome, start Chrome with a bind address:

```bash
CHROME_DEBUG_BIND_ADDR=0.0.0.0 make dev-chrome
```

## 2) Create .env

```bash
cp .env.example .env
```

Set at least:

- `CHROME_DEBUG_HOST` (usually `host.docker.internal` on Docker Desktop)
- `CHROME_DEBUG_PORT` (default `9222`)
- `RABBITMQ_URL` (default works for local Docker RabbitMQ)

Optional (only if you want to override defaults):

- `RABBITMQ_EXCHANGE`
- `RABBITMQ_QUEUE`
- `RABBITMQ_ROUTING_KEY`
- `RABBITMQ_PREFETCH`
- `RABBITMQ_DECLARE_TOPOLOGY`
- `CRAWL_PROMPT_MODE` (`legacy` by default; set to `skill` to use skill-mode prompts)
- `CRAWL_SKILL_NAME` (optional; defaults to `shopee-product-crawler` in skill mode)

## 3) Run database migrations (once)

The worker persists crawl results to SQLite (Turso/libSQL). For local dev, this uses `./out/turso.db`.

```bash
TURSO_SQLITE_PATH=./out/turso.db go run ./cmd/migrate up
```

## 4) Start Worker

```bash
docker compose up --build worker
```

This runs the AMQP worker binary (`/app/worker` from `cmd/worker/main.go`) inside the container.

## Notes

- The worker consumes from RabbitMQ and writes drafts into the SQLite DB under `./out`.
- Docker mounts `./out` into the container, so outputs persist on the host.
- Codex/Gemini auth is stored in `./codex/.codex` and `./gemini/.gemini` (mounted into the container).

## Skill Mode Setup

Skill sources tracked in this repo:

- Codex: `codex/.codex/skills/*/SKILL.md`
- Gemini: `gemini/.gemini/skills/*/SKILL.md`

Maintenance policy:

- Keep Codex and Gemini skill definitions in sync when editing.
- `.agents/skills` and `.gemini/skills` are no longer used in this repo.

Recommended env for the new snapshot-first pipeline:

```bash
export CRAWL_PROMPT_MODE=skill
export CRAWL_SKILL_NAME=shopee-orchestrator-pipeline
```

Note:
- If `CRAWL_SKILL_NAME` is not set, runner may still default to `shopee-product-crawler`.
- Set `CRAWL_SKILL_NAME=shopee-orchestrator-pipeline` explicitly to use the new multi-stage pipeline.

### Local environment

Install repo-tracked skills into your user home:

```bash
make skills-install tool=both
```

This installs:

- Codex skills to `$HOME/.codex/skills`
- Gemini skills with `gemini skills install ... --scope user` and enables them

Equivalent script (if needed):

```bash
./scripts/install-local-skills.sh --tool both
```

Then run from repo root with:

- `CRAWL_PROMPT_MODE=skill`
- `CRAWL_SKILL_NAME=shopee-orchestrator-pipeline`

### Docker environment

Docker uses mounted tool homes:

- Codex: `./codex/.codex` -> `$HOME/.codex` in container
- Gemini: `./gemini/.gemini` -> `$HOME/.gemini` in container

No extra sync step is required. Docker mounts these directories directly, and `scripts/deploy.sh` uploads the same skill directories to remote hosts.

Then set in `.env` (or environment) for `worker` / `runner`:

```bash
CRAWL_PROMPT_MODE=skill
CRAWL_SKILL_NAME=shopee-orchestrator-pipeline
```

## Remote Worker (host Chrome + auth upload)

Use this when running the worker on a remote Ubuntu server with Chrome DevTools on the host.

1) Clone the repo on the server:

```bash
git clone https://github.com/cash-rats/peasydeal-product-miner.git
cd peasydeal-product-miner
```

2) Create `.env` on the server (adjust as needed):

```bash
cp .env.example .env
```

At minimum, set `CHROME_DEBUG_PORT` if you’re not using 9222.

3) Start Chrome DevTools on the host:

```bash
make dev-chrome
```

If you use `network_mode: host` for the worker, Chrome can stay bound to `127.0.0.1`.

4) Login locally for Codex/Gemini, then upload auth to the server:

```bash
make docker-codex-login   # or make docker-gemini-login
make auth-upload env=<name> auth_tool=codex|gemini|both
```

This requires `.env.deploy.<name>` to define `PROD_HOST`, `PROD_USER`, and `PROD_DIR`.

5) Start RabbitMQ + worker:

```bash
docker compose up -d --build rabbitmq worker
```

Notes:
- If Shopee triggers CAPTCHA, keep Chrome running and re-open VNC only to solve it. VNC doesn’t need to stay open all the time; it’s only needed to interact with the browser when a login/CAPTCHA appears. Chrome should stay running in the background so the crawler can reuse the same profile and cookies.
- If you’re using a cloud RabbitMQ instance, set `RABBITMQ_URL` in `.env` and skip any local RabbitMQ service.
