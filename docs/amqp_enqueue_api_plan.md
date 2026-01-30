# AMQP Crawl Enqueue API — Implementation Plan

## Goal

Expose an HTTP API on the long-lived server (`cmd/server/main.go`) that enqueues URL crawl jobs to RabbitMQ using the existing crawlworker message format, so they are consumed by `internal/app/amqp/crawlworker/consumer.go` and processed by `internal/app/amqp/crawlworker/handler.go`.

This API is a **producer only**. The consumer/handler remain in the worker process (`cmd/worker`).

## Feasibility

High. The repo already has:
- RabbitMQ connection/channel provider: `internal/pkg/amqpclient/amqpclient.go`
- Crawl job envelope type: `internal/app/amqp/crawlworker/message.go`
- A worker consumer that reads queue and calls handler: `internal/app/amqp/crawlworker/consumer.go`
- Chi routing + FX wiring patterns for new routes: `internal/router/core.go`, `internal/router/fx/options.go`
- Response helpers: `internal/pkg/render/render.go`

## Proposed Endpoint

### `POST /api/crawl/enqueue`

**Request JSON**
```json
{
  "url": "https://example.com/product/123"
}
```

**Behavior**
- Validate `url` is present and parseable (`net/url`); reject non-http(s) if desired.
- Generate `event_id` server-side as a deterministic hash of the normalized URL (no date bucket), e.g. `urlsha256:<sha256(url)>`.
- Publish `internal/app/amqp/crawlworker.CrawlRequestedEnvelope`:
  - `event_name`: `"crawler/url.requested"` (or empty; consumer allows empty or that exact name)
  - `event_id`: generated/received
  - `ts`: `time.Now().UTC()`
  - `data.url`: request url
  - `data.out_dir`: omitted (worker defaults to `out`)
- Set AMQP properties:
  - `MessageId = event_id` (consumer uses `MessageId`/`CorrelationId` as fallback)
  - `ContentType = "application/json"`
  - `Timestamp = now`

**Response JSON (200)**
```json
{
  "ok": true,
  "event_id": "…",
  "exchange": "events",
  "routing_key": "crawler.url.requested.v1"
}
```

**Error codes**
- `400`: invalid JSON / missing url / invalid url
- `501` or `503`: RabbitMQ disabled (missing `RABBITMQ_URL`) or channel not available
- `502`: publish failed (exchange missing, connection down, etc.)

## Wiring / Modules (FX + chi)

### New domain module
- `internal/app/amqp/enqueue/handler.go`
  - Implements `internal/router.Handler`:
    - `RegisterRoute(r *chi.Mux)` registers `POST /api/crawl/enqueue`
    - `Handle(w, r)` publishes to RabbitMQ
  - Dependencies via `fx.In`:
    - `*config.Config`
    - `*amqp.Channel` (optional)
    - `*zap.SugaredLogger`

- `internal/app/amqp/enqueue/fx/module.go`
  - Provides RabbitMQ via `amqpclient.NewAMQP` (same as worker)
  - Registers route using `router.AsRoute(enqueue.NewHandler)`

### Server entrypoint
- Update `cmd/server/main.go` to include the new enqueue module:
  - `enqueuefx.Module` (and any needed shared amqp providers)

## RabbitMQ Topology Considerations

Publishing will fail if the exchange does not exist. The worker currently declares topology when it starts (`DeclareTopology` flag in `internal/app/amqp/crawlworker/consumer.go`).

Two safe options (pick one):
1) **Publisher also declares exchange** when `cfg.RabbitMQ.DeclareTopology` is true (idempotent `ExchangeDeclare`), so the API can be used even if the worker isn’t running yet.
2) **Operational contract**: worker must start first and declare topology; server only publishes.

Recommendation: option (1) for developer ergonomics.

## Security / Abuse Prevention (Recommended)

This endpoint can trigger arbitrary crawling; protect it.

Minimal options:
- Require an API key header (e.g. `X-Internal-Token`) with config default empty (endpoint returns `401` until set).
- Alternatively restrict to internal network / reverse proxy auth.
- Enforce rate limits (simple in-memory token bucket) if exposed beyond localhost.

## Test Plan

### Unit tests (fast, no RabbitMQ)
- Request validation:
  - missing url → `400`
  - invalid url → `400`
- RabbitMQ disabled:
  - channel nil / `RABBITMQ_URL` empty → `501/503`

### Integration test (optional, gated by env)
- If `RABBITMQ_URL` set:
  - Start an fx app that provides `amqpclient.NewAMQP`
  - Publish via handler (call `ServeHTTP`)
  - Consume one message from queue and assert envelope fields

## Rollout Steps

1) Implement handler + module + server wiring.
2) Run server locally and hit `POST /api/crawl/enqueue`.
3) Confirm worker consumes and persists (`crawlworker_finished` logs; SQLite draft written).
4) Add docs to `README.md` (endpoint usage + required env vars; none required by default except RabbitMQ when enabled).

## Open Questions

1) Should the API accept multiple URLs per request (batch) or single URL only for v1?
