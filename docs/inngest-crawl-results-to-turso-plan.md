# Inngest Crawl Results → Turso SQLite (Feasibility + Implementation Plan)

## Goal

Persist each Inngest crawl run’s output (currently written as a JSON file under an output directory) into Turso (libSQL/SQLite), while keeping the existing file output behavior intact.

## Current State (what we have)

- Crawl execution lives in `internal/app/inngest/crawl/crawl.go` and returns:
  - `out_path` (path to the JSON written under `outDir`)
  - `result` (a `runner.Result` map)
- The runner always writes JSON to disk via `internal/runner/runner.go` (`writeResult(...)`).
- Turso/SQLite connectivity is already present and FX-wired:
  - `db/sqlite.go` provides `db.Conn` as `name:"sqlite"` (optional; disabled unless `TURSO_SQLITE_DSN` or `TURSO_SQLITE_PATH` is set).
  - `db/fx/sqlite_module.go` is included in `cmd/server/main.go`.
- There is already a migrations system (goose) for SQLite under `db/migrations/`.

## Feasibility Assessment

This is feasible with minimal architectural risk because:

- The Turso SQLite connection is already implemented and injected via FX as an optional named dependency (`name:"sqlite"`), which fits the repo’s DI rules.
- The repo already expects “SQLite may be disabled”; the health handler already handles `db.ErrSQLiteDisabled`.
- Adding persistence can be done as a small, isolated DAO + migration + a new Inngest step, without changing the runner’s existing behavior.

Key decisions still needed:

1) **Where to store crawl results**
   - **Option A (recommended):** Create a dedicated table (e.g. `crawl_results`) to store raw crawl output JSON + metadata.
   - **Option B:** Reuse `product_drafts` by mapping crawl outputs into `draft_payload` and mapping runner statuses to draft statuses.
     - This is doable, but it blurs concerns (“crawl result” vs “draft lifecycle”) and forces awkward status mapping.

2) **Behavior when SQLite is disabled**
   - **Default (recommended):** Do not fail the crawl; log and return success, skipping DB persistence.
   - Alternative: treat “SQLite disabled” as a hard error for the Inngest function (would block ingestion in environments without Turso configured).

## Proposed Design (updated: reuse existing schema)

### 1) Reuse existing `product_drafts` table (`db/schema.sql`)

Instead of introducing a new table, persist crawl outputs into the existing Turso table:

- Table: `product_drafts` (defined in `db/schema.sql`, created via goose migration `db/migrations/20260120055907_product_drafts.sql`).
- Data:
  - `draft_payload`: store the full crawl output JSON (the `runner.Result` map) as-is; it already includes `url`, `source`, `captured_at`, `status`, plus product fields.
  - `status`: map runner status to draft lifecycle status (see below).
  - `error`: populate when status is `FAILED` to satisfy the table constraint.

Status mapping proposal:

- runner `status=ok` → `product_drafts.status=READY_FOR_REVIEW`
- runner `status=needs_manual` → `product_drafts.status=READY_FOR_REVIEW` (include `notes` in `draft_payload`)
- runner `status=error` → `product_drafts.status=FAILED` and set `product_drafts.error` (also keep `error` in `draft_payload`)

Idempotency / dedupe proposal:

- Prefer using `input.Event.ID` (when present) as `product_drafts.id` for natural dedupe.
  - If the same event is reprocessed, we can `INSERT ... ON CONFLICT(id) DO UPDATE ...`.
- If `input.Event.ID` is nil, generate a UUID for `id` and always insert a new row.

### 2) Storage layer (as you suggested)

Create a small storage component that owns SQL writes and accepts an injected sqlite connection (Turso).

- Package location suggestion: `internal/app/inngest/productdrafts` (keeps it scoped to the Inngest domain, and matches the existing `product_drafts` concept).
- Public surface:
  - `type ProductDraftStore struct { conn db.Conn; logger *zap.SugaredLogger }`
  - `func NewProductDraftStore(...) *ProductDraftStore`
  - `func (s *ProductDraftStore) UpsertFromCrawlResult(ctx context.Context, in UpsertFromCrawlResultInput) (draftID string, err error)`

Injection:

- Use `db.Conn` with `name:"sqlite" optional:"true"` in `fx.In` params.
- DAO should:
  - If `conn == nil`: treat as “sqlite not wired” and skip (or error based on chosen behavior).
  - If `Exec(...)` returns `db.ErrSQLiteDisabled`: skip (or error based on chosen behavior).

### 3) Write from the Inngest function as a separate step

In `internal/app/inngest/crawl/crawl.go`, add a new Inngest step after `run-crawler`:

- `step.Run(ctx, "persist-crawl-result", func(ctx context.Context) (any, error) { ... })`
- Serialize `r.Result` to JSON (use `encoding/json` with deterministic behavior if needed).
- Extract common metadata from `r.Result` (url/source/status/captured_at) if present.
- Call DAO insert/upsert.

Error policy:

- If SQLite is enabled and the insert fails for “real” reasons, return `inngestgo.NoRetryError(err)` (consistent with how crawling errors are handled today).
- If SQLite is disabled, do not fail (recommended default).

## Implementation Plan (incremental)

### Step 0 — Confirm decisions

- Confirm we’re persisting into `product_drafts` (per `db/schema.sql`) and not adding a new table.
- Confirm behavior when SQLite is disabled (skip vs fail).
- Confirm desired idempotency semantics when `input.Event.ID` is present (upsert vs do-nothing).

### Step 1 — Ensure schema exists in Turso

- Ensure the goose migration `db/migrations/20260120055907_product_drafts.sql` has been applied to the target Turso DB (or equivalently that `product_drafts` exists as in `db/schema.sql`).

### Step 2 — Add storage

- Add `internal/app/inngest/productdrafts/store.go` with:
  - `NewProductDraftStore` (FX-provided)
  - `UpsertFromCrawlResult(...)` performing insert/upsert into `product_drafts`

### Step 3 — Wire storage via FX

- Update `internal/app/inngest/fx/module.go`:
  - `fx.Provide(productdrafts.NewProductDraftStore)`

### Step 4 — Persist from crawl function

- Update `internal/app/inngest/crawl/crawl.go`:
  - Inject DAO into `CrawlFunction`
  - Add `persist-crawl-result` step after `run-crawler`
  - Keep current disk output behavior unchanged

### Step 5 — Add minimal coverage + docs

- Add a small unit test for DAO behavior when SQLite is disabled (expects `db.ErrSQLiteDisabled` path is treated according to the decision).
- Add a short README note:
  - “Crawl results are written to disk and (optionally) to Turso when `TURSO_SQLITE_*` is configured and built with `-tags=sqlite`.”

## Notes / Risks

- SQLite write throughput: storing full JSON per crawl is fine for moderate volume; if volume grows, we can split “metadata columns + blob JSON” and/or store only normalized fields.
- Schema evolution: keeping `result_json` as the source of truth makes it easy to add columns later without breaking compatibility.
- Build tag: Turso driver code is present, but local SQLite usage may depend on `-tags=sqlite` per README; we should keep runtime behavior clear (skip writes cleanly when not enabled).
