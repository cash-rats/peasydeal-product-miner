# Persist `QUEUED_FOR_DRAFT` Before AMQP Publish (and move ProductDraftStore)

## Goal

When a client calls `POST /v1/crawl/enqueue`, we want to **persist a draft row in SQLite/Turso** before publishing the crawl request to RabbitMQ. The persisted row acts as the “job record” and starts with status `QUEUED_FOR_DRAFT`.

Separately, the `ProductDraftStore` code previously lived under `internal/app/inngest/dao/` even though the repo is moving away from `inngest/`. This change relocates that store into the AMQP domain.

## Summary of Changes

### 1) Enqueue now writes to SQLite first (best-effort)

- File: `internal/app/amqp/enqueue/handler.go`
- Behavior: after validating the URL and generating `event_id`, the handler attempts:
  - `productdrafts.ProductDraftStore.UpsertQueuedForDraft(...)`
  - If it fails, the handler logs the error and **still publishes to RabbitMQ**.
- Data written:
  - `product_drafts.status = 'QUEUED_FOR_DRAFT'`
  - `product_drafts.event_id = eventIDFromURL(url)` (same value used as RabbitMQ `message_id`)
  - `product_drafts.draft_payload` includes at minimum `{ "url": "<url>" }` and includes `"source"` when it can be derived (`"shopee"` / `"taobao"`).

Rationale for best-effort: SQLite/Turso is optional in this repo; enqueueing should not be blocked when it’s disabled or temporarily unavailable. (RabbitMQ publish is still treated as required.)

### 2) `ProductDraftStore` moved out of `inngest/`

Moved:
- From: `internal/app/inngest/dao/store.go`
- To: `internal/app/amqp/productdrafts/store.go`

Moved test:
- From: `internal/app/inngest/dao/store_test.go`
- To: `internal/app/amqp/productdrafts/store_test.go`

New FX module:
- `internal/app/amqp/productdrafts/fx/module.go` provides `productdrafts.NewProductDraftStore`

Wiring updates:
- `cmd/server/main.go` includes `internal/app/amqp/productdrafts/fx.Module`
- `cmd/worker/main.go` includes `internal/app/amqp/productdrafts/fx.Module`
- Inngest code that still writes crawl results now imports the store from the new location.

## Notes / Operational Requirements

- SQLite/Turso is still optional; when disabled, store operations return `db.ErrSQLiteDisabled` and the service continues running.
- Ensure the `product_drafts` migrations are applied to your Turso/SQLite DB:
  - `go run ./cmd/migrate up`
- The schema already supports `QUEUED_FOR_DRAFT` and `event_id`:
  - `db/migrations/20260120055907_product_drafts.sql`
  - `db/migrations/20260127053509_product_drafts_event_id.sql`
