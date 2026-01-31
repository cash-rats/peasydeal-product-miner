# Plan: `GET /v1/product-drafts/{id}` (fetch draft by id)

## Goal
Add a read-only API endpoint:

- `GET /v1/product-drafts/{id}`

It retrieves a single product draft by `id` from Turso/SQLite `product_drafts`, parses the `draft_payload` JSON string, and returns the parsed object in the response.

## Current repo state (why this is feasible)
- `product_drafts` table already exists in `db/schema.sql` (includes `draft_payload` as JSON text, status, timestamps, publish fields).
- SQL query already exists via sqlc: `GetProductDraft` in `db/sqlc/product_drafts.sql.go`.
- Server already uses chi + FX and has route wiring conventions (`internal/router.Handler` + `router.AsRoute`).
- SQLite can be “disabled” (no `TURSO_DATABASE_URL`), and the app is designed to still boot; this endpoint should follow that convention.

## Proposed API contract

### Path
- `GET /v1/product-drafts/{id}`

Notes:
- Treat `{id}` as an **opaque string** (not necessarily UUID). Some legacy IDs may contain `:` (e.g. `urlsha256:<hash>`), so avoid UUID-only validation.

### Success response (200)
Return draft metadata plus parsed payload:

```json
{
  "id": "string",
  "status": "QUEUED_FOR_DRAFT|CRAWLING|DRAFTING|READY_FOR_REVIEW|PUBLISHED|FAILED|REJECTED|FOUND",
  "draft": {
    "captured_at": "2026-01-31T08:34:23.026Z",
    "currency": "TWD",
    "description": "…",
    "images": ["https://…"],
    "price": "1",
    "source": "shopee",
    "status": "ok",
    "title": "…",
    "url": "https://…"
  },
  "error": "string|null",
  "created_by": "string|null",
  "created_at_ms": 0,
  "updated_at_ms": 0,
  "published_at_ms": 0,
  "published_product_id": "string|null"
}
```

Implementation detail: the `draft` field is the JSON-unmarshaled form of `draft_payload` (stored as TEXT).

### Error responses
- `400` if `{id}` is missing/blank after trimming
- `404` if draft id not found
- `503` if Turso/SQLite is disabled (no DB available), to match the “don’t block startup” convention
- `500` for unexpected DB errors or JSON parsing issues (strict)

For consistency with existing endpoints, use:
- `internal/pkg/render.ChiJSON` for success bodies
- `internal/pkg/render.ChiErr` for error bodies

Suggested `render.ChiErr` messages (simple + consistent with existing style):
- `400`: `"missing id"`
- `404`: `"not found"`
- `503`: `"sqlite disabled"`
- `500` (db): `"failed to fetch product draft"`
- `500` (json parse): `"invalid draft payload"`

## Implementation outline (code structure)

### 1) New HTTP domain module: `internal/app/productdrafts`
Rationale:
- Avoid coupling HTTP endpoints to `internal/app/amqp/productdrafts` (which is currently a worker/persistence-oriented package).
- Keep the “API surface” for drafts under a stable domain package that can later host list/approve/reject endpoints.

New files (proposed):
- `internal/app/productdrafts/get_by_id_handler.go`
- `internal/app/productdrafts/fx/module.go`

### 2) Data access
Use the existing sqlc query:
- `sqlcdb.New(sqliteDB).GetProductDraft(ctx, id)`

Where `sqliteDB` is injected via FX:
- `*sqlx.DB` named `"sqlite"` (from `dbfx.SQLiteModule`)

Behavior when SQLite is disabled:
- If `*sqlx.DB` is `nil`, immediately return `503`.

### 3) Handler behavior
- Register route: `r.Get("/v1/product-drafts/{id}", h.Handle)`
- Read `{id}` via `chi.URLParam(r, "id")`, `strings.TrimSpace`
- Query draft:
  - map `sql.ErrNoRows` → `404`
- Parse JSON:
  - `json.Unmarshal([]byte(row.DraftPayload), &draft)` into `map[string]any`
  - if JSON invalid (unexpected, given DB `CHECK(json_valid)`): return `500` and log the parse error
- Return response object described above

## Wiring (FX)
- Provide the handler via `router.AsRoute(NewGetByIDHandler)` inside `internal/app/productdrafts/fx/module.go`
- Add `productdraftsfx.Module` to `cmd/server/main.go`

## Testing plan
Add a focused handler test that covers:
- `200` happy path: insert a draft row with the sample `draft_payload`, call HTTP GET, assert response contains parsed `draft` object fields.
- `404` when id doesn’t exist.
- `503` when sqlite is disabled (no Turso config): handler should respond `{"error":"sqlite disabled"}` (exact string TBD) with status `503`.

Preferred approach:
- Reuse the project’s existing SQLite test patterns (see existing `*_test.go` in `internal/app/*` and `db/sqlite_test.go`).

## Rollout notes
- No DB migration required (table/query already exist).
- Backward compatibility: allow ids with `:`; avoid UUID-only parsing.

## Decisions (confirmed)
- Response includes metadata + parsed `draft` object.
- JSON parsing is strict: invalid `draft_payload` returns `500`.
