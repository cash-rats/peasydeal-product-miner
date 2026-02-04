# peasydeal-product-miner — Server APIs

Source of truth: `docker-compose.yml` (service `server`) + Go handlers under `internal/app/**`.

## Base URL
- Local (docker-compose): `http://localhost:${APP_PORT:-3012}`
- Server binds to `APP_ADDR`/`APP_PORT` (defaults: `0.0.0.0:3012` in compose).

## Endpoints

### 1) Health Check
**Method**: `GET`  
**Path**: `/health`

**Response (200)**
```json
{
  "ok": true,
  "sqlite": {
    "enabled": true,
    "status": "ok"
  }
}
```

**Response (503)**  
Returned when SQLite is enabled but query fails.
```json
{
  "ok": false,
  "sqlite": {
    "status": "error",
    "error": "..."
  }
}
```

**Notes**
- If SQLite is not enabled, response still `200` with `"enabled": false`.

---

### 2) Get Product Draft by ID
**Method**: `GET`  
**Path**: `/v1/product-drafts/{id}`

**Response (200)**
```json
{
  "id": "string",
  "status": "string",
  "draft": {},
  "error": "string|null",
  "created_by": "string|null",
  "created_at_ms": 0,
  "updated_at_ms": 0,
  "published_at_ms": 0,
  "published_product_id": "string|null"
}
```

**Errors**
- `400` — `"missing id"`
- `503` — `"sqlite disabled"`
- `404` — `"not found"`
- `500` — `"failed to fetch product draft"`
- `500` — `"invalid draft payload"`

**Notes**
- Requires SQLite (Turso) to be enabled and connected.
- `draft` field is JSON parsed from stored payload.

---

### 3) Enqueue Crawl Request (RabbitMQ)
**Method**: `POST`  
**Path**: `/v1/crawl/enqueue`

**Request Body**
```json
{
  "url": "https://example.com/..."
}
```

**Response (200)**
```json
{
  "ok": true,
  "event_id": "urlsha256:<hash>",
  "id": "<draft_id_or_empty>"
}
```

**Validation Errors (400)**
- `"invalid json"`
- `"missing url"`
- `"invalid url"`
- `"url must be http(s)"`
- `"invalid url host"`
- `"unsupported url domain (supported: shopee, taobao)"`

**Service Errors**
- `503` — `"rabbitmq disabled"`
- `502` — `"rabbitmq exchange declare failed: <exchange>"`
- `502` — `"failed to publish message"`
- `500` — `"failed to encode message"`

**Notes**
- Supported domains: `taobao.com`, `*.taobao.com`, `shopee.com`, `*.shopee.com`, and regional `shopee.<tld>`.
- When SQLite is enabled, it attempts to upsert a “queued” draft and returns `id` if successful.
- RabbitMQ settings are driven by config/env (see below).

---

### 4) Inngest Webhook/Serve Endpoint
**Methods**: `GET`, `POST`, `PUT`  
**Path**: `/api/inngest` (default)

**Response (501)**
```json
{ "error": "inngest disabled: set INNGEST_APP_ID to enable" }
```

**Notes**
- Path can be overridden via `INNGEST_SERVE_PATH`.
- If `INNGEST_APP_ID` is empty, the endpoint returns `501`.
- Actual behavior handled by Inngest SDK once enabled.

---

## Cross-Cutting Behavior
- CORS enabled for production/preview and for local dev (`localhost:5173`).
- `OPTIONS /*` returns `204` when CORS enabled.
- Request logging includes method, path, status, bytes, duration.

## Relevant Environment / Config (server)
- `APP_ADDR`, `APP_PORT`
- `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `RABBITMQ_ROUTING_KEY`, `RABBITMQ_DECLARE_TOPOLOGY`, `RABBITMQ_PREFETCH`
- `INNGEST_APP_ID`, `INNGEST_SIGNING_KEY`, `INNGEST_SERVE_HOST`, `INNGEST_SERVE_PATH`, `INNGEST_DEV`
- `TURSO_SQLITE_DSN`, `TURSO_SQLITE_TOKEN`, `TURSO_SQLITE_PATH`, `TURSO_SQLITE_DRIVER`

---

## Files / Handlers
- Health: `internal/app/health/handler.go`
- Product Drafts: `internal/app/productdrafts/get_by_id_handler.go`
- Enqueue: `internal/app/amqp/enqueue/handler.go`
- Inngest: `internal/app/inngest/handler.go`
