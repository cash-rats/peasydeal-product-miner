# PeasyDeal — Product Drafts (Turso/SQLite) Table Schema

This schema stores the **draft lifecycle** and all **pipeline state** in Turso (libSQL/SQLite).

## Statuses

Allowed `status` values:
- `FOUND`
- `QUEUED_FOR_DRAFT`
- `CRAWLING`
- `DRAFTING`
- `READY_FOR_REVIEW`
- `PUBLISHED`
- `FAILED`
- `REJECTED`

Transition rules (enforced in application logic unless you add DB triggers):
- `FOUND → QUEUED_FOR_DRAFT`
- `QUEUED_FOR_DRAFT → CRAWLING`
- `CRAWLING → DRAFTING`
- `DRAFTING → READY_FOR_REVIEW`
- `READY_FOR_REVIEW → PUBLISHED`
- Any non-terminal → `FAILED`
- `FOUND|QUEUED_FOR_DRAFT|CRAWLING|DRAFTING → REJECTED`
- `FAILED → QUEUED_FOR_DRAFT` (retry)
- `REJECTED` typically does not flow back (unless manual override)

## Table: `product_drafts`

This version stores the draft fields in a single JSON column (`draft_payload`), but exposes common fields as **generated columns** so you can filter/sort/index them without duplicating data.

```sql
CREATE TABLE IF NOT EXISTS product_drafts (
  id TEXT PRIMARY KEY, -- uuid

  status TEXT NOT NULL CHECK (status IN (
    'FOUND',
    'QUEUED_FOR_DRAFT',
    'CRAWLING',
    'DRAFTING',
    'READY_FOR_REVIEW',
    'PUBLISHED',
    'FAILED',
    'REJECTED'
  )),

  -- Draft payload (minimal v1)
  -- {
  --   "url": "string",
  --   "source": "shopee"|"taobao"|null,
  --   "title": "string|null",
  --   "description": "string|null",
  --   "images": ["https://..."]|null,
  --   "variant_images": [{"name":"blue","image":"https://..."}]|null,
  --   "currency": "TWD"|...|null,
  --   "price": "199.00"|199.00|null
  -- }
  draft_payload TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(draft_payload)),

  -- Queryable fields extracted from JSON (avoid duplication at write time).
  url TEXT GENERATED ALWAYS AS (json_extract(draft_payload, '$.url')) STORED,
  source TEXT GENERATED ALWAYS AS (json_extract(draft_payload, '$.source')) STORED,
  title TEXT GENERATED ALWAYS AS (json_extract(draft_payload, '$.title')) STORED,
  description TEXT GENERATED ALWAYS AS (json_extract(draft_payload, '$.description')) STORED,
  currency TEXT GENERATED ALWAYS AS (json_extract(draft_payload, '$.currency')) STORED,
  price TEXT GENERATED ALWAYS AS (CAST(json_extract(draft_payload, '$.price') AS TEXT)) STORED,

  error TEXT NULL,

  created_by TEXT NULL,

  created_at_ms INTEGER NOT NULL DEFAULT (unixepoch('now') * 1000),
  updated_at_ms INTEGER NOT NULL DEFAULT (unixepoch('now') * 1000),

  published_at_ms INTEGER NULL,
  published_product_id TEXT NULL, -- Supabase product UUID

  CHECK (url IS NOT NULL AND length(trim(url)) > 0),
  CHECK (source IS NULL OR source IN ('shopee', 'taobao')),
  CHECK (currency IS NULL OR length(currency) = 3),
  CHECK (json_type(draft_payload, '$.images') IS NULL OR json_type(draft_payload, '$.images') = 'array'),
  CHECK (json_type(draft_payload, '$.variant_images') IS NULL OR json_type(draft_payload, '$.variant_images') = 'array'),
  CHECK (status != 'FAILED' OR (error IS NOT NULL AND length(trim(error)) > 0))
);

CREATE INDEX IF NOT EXISTS idx_product_drafts_status_updated
  ON product_drafts(status, updated_at_ms DESC);

CREATE INDEX IF NOT EXISTS idx_product_drafts_url
  ON product_drafts(url);

CREATE INDEX IF NOT EXISTS idx_product_drafts_source
  ON product_drafts(source);

CREATE INDEX IF NOT EXISTS idx_product_drafts_creator_created
  ON product_drafts(created_by, created_at_ms DESC);

-- Auto-touch updated_at_ms on any update.
CREATE TRIGGER IF NOT EXISTS trg_product_drafts_touch_updated_at
AFTER UPDATE ON product_drafts
FOR EACH ROW
BEGIN
  UPDATE product_drafts
  SET updated_at_ms = (unixepoch('now') * 1000)
  WHERE id = NEW.id;
END;
```
