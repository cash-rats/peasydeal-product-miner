# PeasyDeal — “AI Product Drafts” API Proposal (Updated: Turso Drafts → Supabase Publish)

## 1) Background

PeasyDeal’s team is small (3 people). Two time-consuming tasks are:
1) **Finding products** to list
2) **Collecting materials** (title, specs, images, variants, SEO copy) and **creating the product** in the admin console

This proposal focuses on an MVP that automates #2 heavily and sets up a foundation to help with #1 later:

> **Paste a product URL → Generate a “Product Draft” → Human review → One-click create product**

The admin console will be built with **Refine**; the draft flow integrates via Refine’s custom request hooks (`useCustomMutation`, `useCustom`) and optionally realtime updates via `liveProvider`.

---

## 2) Goals & Non-goals

### Goals (MVP)
- Create a server-side workflow that:
  - accepts a **product URL** (or supplier SKU later) and optional hints
  - extracts product data (raw)
  - uses an LLM to map data into **PeasyDeal’s product schema**
  - produces a **Product Draft** object for human review/import
- Add a Refine UI entry point:
  - `/products/ai-import` (page) or “Import from URL” modal
  - status/progress updates
  - review & approve → create actual product record(s)

### Non-goals (MVP)
- Fully automated publishing without review
- Full product sourcing/trend discovery automation (Phase 2/3)
- Perfect extraction for every site (start with limited allowlist + improve iteratively)

---

## 3) Storage Architecture (Updated)

### Drafts live in Turso
All draft lifecycle data (status updates, extraction payloads, validation errors, retries) live in **Turso (libSQL/SQLite)**.

### Published products live in Supabase
Only when an admin **approves** a draft do we create real records in **Supabase/Postgres** (`products`, `product_variants`, `product_images`) and link the draft to `approved_product_id`.

This reduces Supabase write volume and keeps “noisy” pipeline state out of the primary DB.

---

## 4) Admin UX (Refine Integration)

### Entry points
**Option A: Dedicated page**
- Route: `/products/ai-import`
- UI: input URL + optional hints (category, target language, margin assumptions, vendor, tags)

**Option B: Modal in Products list**
- “Import from URL” button opens modal form

### How Refine calls the backend
- **Create Draft**: `useCustomMutation` → POST
- **Poll Draft**: `useCustom` → GET (poll) or realtime later via `liveProvider`
- **Approve**: POST approve (publishes to Supabase)

---

## 5) System Overview

### High-level flow
1) Admin submits URL
2) API creates a `product_draft` record in **Turso** (`status=QUEUED`)
3) Background job runs:
   - Extract → Normalize → LLM draft → Validate
4) Draft becomes `READY_FOR_REVIEW`
5) Admin reviews, edits if needed, then approves
6) API creates real Supabase `products` / `product_variants` / `product_images` and marks draft `APPROVED`

### Key design principle
**Drafts are separate from Products.**
Drafts are a staging area for automation and human QA, preventing incorrect data from polluting production tables.

---

## 6) Data Model (Updated for Turso / SQLite)

### Table: `product_drafts` (Turso)
Suggested columns:
- `id` (TEXT uuid, pk)
- `status` (TEXT):
  `QUEUED | EXTRACTING | DRAFTING | VALIDATING | READY_FOR_REVIEW | APPROVED | FAILED | CANCELLED`
- `source_url` (TEXT)
- `input_hints` (TEXT JSON) — user-provided hints (category, language, etc.)
- `raw_extraction` (TEXT JSON) — unmodified extraction result (for debugging)
- `draft_payload` (TEXT JSON) — normalized PeasyDeal product schema for review/import
- `validation_errors` (TEXT JSON)
- `error_message` (TEXT)
- `created_by` (TEXT) — admin user id
- `created_at`, `updated_at` (TEXT / INTEGER)
- `approved_at` (TEXT)
- `approved_product_id` (TEXT, nullable) — Supabase product UUID
- `cost` (TEXT JSON, optional)

**JSON validity (optional but recommended)**
Use SQLite JSON1 `json_valid()` in `CHECK()` constraints, e.g. `CHECK(json_valid(draft_payload))`. citeturn0search2turn0search9

### Table: `product_draft_events` (optional, Turso)
Append-only log for auditing:
- `id`, `draft_id`
- `event_type` (STATUS_CHANGED, RETRIED, EDITED, APPROVED, FAILED)
- `payload` (TEXT JSON)
- `created_at`, `created_by`

### Table: `product_draft_sources` (optional, Phase 2, Turso)
Track evidence/citations:
- `draft_id`
- `field_path` (e.g. `variants[0].price`)
- `source_url`
- `source_snippet` (TEXT) / `selector` (TEXT)

---

## 7) API Specification (MVP)

> Base path examples use `/admin/ai/*` but can be adjusted to match your routing.

### 7.1 Create draft (writes to Turso)
**POST** `/admin/ai/product-drafts`

Request body:
```json
{
  "source_url": "https://example.com/product/123",
  "hints": {
    "language": "zh-TW",
    "category_id": 5710,
    "vendor": "example-vendor",
    "target_margin": 0.35
  }
}
```

Response:
```json
{
  "draft_id": "uuid",
  "status": "QUEUED"
}
```

### 7.2 Get draft (reads from Turso)
**GET** `/admin/ai/product-drafts/{draft_id}`

Response:
```json
{
  "draft_id": "uuid",
  "status": "READY_FOR_REVIEW",
  "source_url": "https://example.com/product/123",
  "draft_payload": { "...peasydealProductSchema..." },
  "validation_errors": [],
  "error_message": null,
  "updated_at": "2026-01-03T05:12:00Z"
}
```

### 7.3 List drafts (admin queue, reads from Turso)
**GET** `/admin/ai/product-drafts?status=READY_FOR_REVIEW&limit=50`

Response:
```json
{
  "items": [
    { "draft_id": "...", "status": "...", "source_url": "...", "updated_at": "..." }
  ],
  "next_cursor": null
}
```

### 7.4 Retry a failed draft (updates in Turso)
**POST** `/admin/ai/product-drafts/{draft_id}/retry`

### 7.5 Approve draft → publish to Supabase
**POST** `/admin/ai/product-drafts/{draft_id}/approve`

Request:
```json
{
  "final_payload": { "...optional edited payload..." }
}
```

Behavior:
- Reads the latest draft from Turso
- Uses `final_payload` if provided, else `draft_payload`
- Transactionally creates product records in **Supabase**
- Updates the Turso draft: `status=APPROVED`, `approved_product_id`, `approved_at`

Response:
```json
{
  "draft_id": "uuid",
  "status": "APPROVED",
  "product_id": "uuid"
}
```

### 7.6 Cancel draft (optional)
**POST** `/admin/ai/product-drafts/{draft_id}/cancel`

---

## 8) Draft Payload Schema (Core Concept)

The `draft_payload` should match your internal product creation needs. Example shape:

```json
{
  "title": "string",
  "subtitle": "string",
  "description_html": "string",
  "bullet_points": ["string"],
  "brand": "string",
  "category_id": 123,
  "tags": ["tag1", "tag2"],
  "seo": {
    "slug": "string",
    "meta_title": "string",
    "meta_description": "string"
  },
  "attributes": {
    "material": "string",
    "dimensions": "string",
    "pet_size": ["S", "M", "L"]
  },
  "images": [
    { "url": "https://...", "alt": "string", "position": 1 }
  ],
  "variants": [
    {
      "sku": "string",
      "option_values": { "color": "Black", "size": "M" },
      "price": 499,
      "compare_at_price": 699,
      "inventory": 50,
      "weight_grams": 300
    }
  ]
}
```

### Why “Structured Outputs” matters
When generating `draft_payload` from an LLM, require the model to comply with a **JSON Schema** so responses are machine-valid and required fields aren’t omitted.

---

## 9) Job Processing Pipeline (Implementation Notes)

### Step A — Extraction
- Fetch HTML from `source_url`
- Extract: title, price, variants/options, specs table, images, shipping notes, etc.
- Store as `raw_extraction` in Turso

**Hard requirements**
- SSRF protection (block internal IP ranges, disallow `file://`, etc.)
- Domain allowlist for MVP (start with 3–10 sources)

### Step B — Normalize
Map extraction output into a consistent intermediate format: `normalized_product`

### Step C — LLM Draft
- Prompt LLM with normalized data + hints
- Require structured output (JSON schema)
- Output → `draft_payload`

### Step D — Validate
Rules:
- required fields present
- variants completeness (price, SKU or computed SKU, option values)
- image URLs valid
- category mapping valid

If validation fails:
- set `status=FAILED` or `READY_FOR_REVIEW` with warnings

### Step E — Approve/Create Product (Supabase)
- Transactionally create product + variants + images in Supabase
- Mark Turso draft approved and link `approved_product_id`

---

## 10) Security, Compliance, and Ops

- **Auth**: restrict endpoints to admin users (JWT/session)
- **Rate limiting**: per-admin + per-domain
- **Audit log**: `product_draft_events` in Turso
- **Cost tracking**: store token usage/extraction cost in `product_drafts.cost`
- **Observability**: trace id per draft; log step timings; store error codes
- **Data retention**: TTL cleanup (e.g. delete drafts after 7–30 days) to keep Turso small
- **Large blobs**: store large extraction/HTML in object storage and only keep pointers in Turso

---

## 11) Rollout Plan

### Phase 1 (MVP)
- Create draft endpoint + background job
- Store drafts in Turso
- Refine page with polling + review + approve
- Approve publishes to Supabase

### Phase 2
- Evidence/citations per field (`product_draft_sources`)
- Rehost images to CDN/R2
- Better category auto-mapping

### Phase 3
- Product discovery automation
- Bulk draft creation
- A/B copy generation, localization

---

## 12) Acceptance Criteria (MVP)
- Admin can paste a URL and get a draft within ~1–3 minutes for supported domains
- Draft includes: title, description, images, at least one variant with price
- Admin can edit draft fields and approve
- Approve creates real product records in Supabase without manual copy/paste
- Failed drafts are diagnosable (error message + raw extraction stored or referenced)
