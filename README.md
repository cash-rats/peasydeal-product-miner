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

## 3) Run database migrations (once)

The worker persists crawl results to SQLite (Turso/libSQL). For local dev, this uses `./out/turso.db`.

```bash
TURSO_SQLITE_PATH=./out/turso.db go run ./cmd/migrate up
```

## 4) Start RabbitMQ + Worker

```bash
docker compose up --build rabbitmq worker
```

This runs the AMQP worker binary (`/app/worker` from `cmd/worker/main.go`) inside the container.

## Notes

- The worker consumes from RabbitMQ and writes drafts into the SQLite DB under `./out`.
- Docker mounts `./out` into the container, so outputs persist on the host.
- Codex/Gemini auth is stored in `./codex/.codex` and `./gemini/.gemini` (mounted into the container).
