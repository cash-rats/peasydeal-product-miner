# PeasyDeal — AI Product Drafts (Refine) Implementation Plan (Turso Drafts → Supabase Publish)

This document implements `docs/peasydeal-ai-product-drafts-proposal.md` as a concrete, incremental delivery plan for the Refine admin console.

## Decision: Store drafts in Turso, publish approved products to Supabase

**MVP architecture**
- **Draft lifecycle + pipeline state** live in **Turso (libSQL / SQLite)**:
  `QUEUED → EXTRACTING → DRAFTING → VALIDATING → READY_FOR_REVIEW` (+ terminal: `APPROVED | FAILED | CANCELLED`)
- **Published products** (source-of-truth) live in **Supabase/Postgres** and are created only at **Approve** time.

**Why**
- Drafts are **write-heavy** (status updates, retries, raw extraction payloads).
- Products are **audit-critical** and should remain in your primary DB (Supabase).
- This keeps your Supabase free tier from being consumed by noisy draft traffic.

**Turso connectivity**
- Retrieve DB URL: `turso db show --url <db-name>`
- Create auth token: `turso db tokens create <db-name>` (optionally `--read-only`, `--expiration`) citeturn0search7turn0search1

---

## Recommendation: Orchestration (n8n vs Inngest)

### Use Inngest for the core pipeline (recommended for MVP)
- Code-first workflow orchestration that lives with the repo.
- Clear step boundaries matching the draft statuses.
- Retries/backoff, idempotency, concurrency limits, and step-level observability.
- Easier to enforce SSRF protection, allowlists, and auth consistently in code.

### Use n8n only for optional integrations (post-MVP)
- Slack notifications, Sheets export, vendor connectors, etc.
- Keep it out of the critical path.

**MVP choice:** Inngest for the AI draft pipeline; keep n8n out of the critical path.

---

## UX: Dedicated Page (Simplest Operator Flow)

### Route
- `/products/ai-import`

### Single-page flow (minimal steps)
1. Paste **product URL** + optional hints (category, target language, margin assumptions, vendor, tags).
2. Click **Generate Draft**.
3. See **progress + status** (`QUEUED/EXTRACTING/DRAFTING/VALIDATING`).
4. When ready (`READY_FOR_REVIEW`), see a **review form pre-filled by AI** (only required fields up front).
5. Click **Approve & Create Product**.
6. Backend **creates product records in Supabase** and returns `product_id`.
7. Redirect to the created product page (edit/show) for final tweaks.

**Design principle:** keep everything on one page; avoid wizards/modals for MVP.

---

## Delivery Plan: Break the Feature into Small, Shippable Steps

Each step is independently doable and deployable.

### Step 0 — Turso setup (credentials + connectivity)
- Create a Turso database for drafts (e.g. `peasydeal-drafts`).
- Store secrets:
  - `TURSO_DATABASE_URL`
  - `TURSO_AUTH_TOKEN` (server-only)
- Decide SDK approach:
  - Prefer Turso’s Go quickstart path for server access citeturn0search7

**Deliverable:** service can connect to Turso and run a test query.

---

### Step 1 — Drafts storage + API skeleton (Turso, no AI yet)
Create `product_drafts` in **Turso**.

**SQLite/JSON note**
- Store JSON payloads as **TEXT**.
- (Optional but recommended) add constraints like `CHECK(json_valid(input_hints))` etc. using SQLite JSON1 `json_valid()` citeturn0search2turn0search9

**Suggested columns**
- `id` (TEXT UUID, pk)
- `status` (TEXT)
- `source_url` (TEXT)
- `input_hints` (TEXT JSON)
- `raw_extraction` (TEXT JSON)
- `draft_payload` (TEXT JSON)
- `validation_errors` (TEXT JSON)
- `error_message` (TEXT)
- `created_by` (TEXT)
- `created_at` (TEXT / INTEGER unixms)
- `updated_at` (TEXT / INTEGER unixms)
- `approved_at` (TEXT, nullable)
- `approved_product_id` (TEXT, nullable) — Supabase product UUID
- `cost` (TEXT JSON, optional)

**Indexes (practical)**
- `(status, updated_at DESC)` for admin queue.
- `(created_by, created_at DESC)`.

**Endpoints (can be stubbed initially)**
- `POST /admin/ai/product-drafts` (create in Turso)
- `GET /admin/ai/product-drafts/{draft_id}` (read from Turso)
- `GET /admin/ai/product-drafts?status=READY_FOR_REVIEW&limit=50` (list from Turso)
- `POST /admin/ai/product-drafts/{draft_id}/approve` (**publish to Supabase**)
- `POST /admin/ai/product-drafts/{draft_id}/retry`
- `POST /admin/ai/product-drafts/{draft_id}/cancel`

**Deliverable:** end-to-end “create draft + poll status” works even with fake payloads.

---

### Step 2 — Refine page MVP (polling + basic display)
- Add `/products/ai-import` page:
  - URL input + “Generate Draft”
  - Poll draft status every N seconds
  - Display status + raw `draft_payload` (temporary JSON viewer is fine)

**Deliverable:** UI can drive the flow and show progress.

---

### Step 3 — Background job infrastructure (Inngest)
- On draft creation, trigger an Inngest function/event.
- Implement step skeleton that updates `product_drafts.status` in **Turso**.
- Add guardrails immediately:
  - domain allowlist for MVP (start with 3–10 sources)
  - SSRF protection (block internal IP ranges, disallow `file://`, etc.)
  - fetch timeout + max HTML size

**Deliverable:** real async processing path exists; still can emit placeholder payload.

---

### Step 4 — Extraction (deterministic, diagnosable)
- Fetch HTML from `source_url` and extract the proposal’s suggested fields (title, price, variants/options, specs table, images, shipping notes where relevant).
- Store unmodified result to `raw_extraction` for debugging.

**Cost control option (recommended)**
If extraction output can be large:
- store big blobs (HTML / large JSON) in object storage (R2/S3),
- keep only a pointer in Turso (`raw_extraction_ref` inside `raw_extraction` or a separate column).

**Deliverable:** supported URLs consistently produce structured extraction output.

---

### Step 5 — Normalize + Validate (still no LLM required)
- Normalize extraction into a stable intermediate shape (e.g. `normalized_product`).
- Validate minimum completeness (e.g. title + at least 1 image + at least 1 variant candidate).

**Deliverable:** stable contract for the LLM step; fewer prompt surprises.

---

### Step 6 — LLM Drafting (structured output + business validation)
- Define a JSON Schema for `draft_payload` (matching the proposal’s “Draft Payload Schema”).
- Prompt with `normalized_product` + hints; require structured output.
- Validate:
  - schema validity
  - required business rules (variants completeness, image URLs valid, category mapping valid)

Set:
- `READY_FOR_REVIEW` for drafts a human can finish
- `FAILED` for hard failures (with actionable `error_message`)

**Deliverable:** draft becomes human-reviewable with minimal manual filling.

---

### Step 7 — Review UI (minimum human effort)
- Replace JSON viewer with a compact review form prioritizing required fields:
  - title, category, images, variants (sku/options/price)
  - optional: description_html, bullets, SEO
- Highlight missing/invalid fields; keep edit friction low.

**Deliverable:** human can “fix the last 10%” quickly.

---

### Step 8 — Approve → Create actual product records (Supabase)
- Implement transactional create in Supabase:
  - `products`
  - `product_variants`
  - `product_images`
- Update Turso draft:
  - `status=APPROVED`
  - `approved_product_id`
  - `approved_at`

**Deliverable:** one-click import into real product tables works.

---

### Step 9 — Operational hardening (post-MVP but important)
- Retry: `POST /admin/ai/product-drafts/:id/retry`
- Cancel: `POST /admin/ai/product-drafts/:id/cancel`
- Auth: restrict all endpoints to admin users (JWT/session).
- Rate limiting per admin + per domain; concurrency caps in Inngest.
- Observability: trace id per draft; log step timings; store error codes.
- Optional audit log table `product_draft_events` (in Turso) for status changes/retries/approvals.
- Optional “evidence/citations” table `product_draft_sources` (Phase 2).
- Optional: realtime updates via `liveProvider` (replace polling).

---

## Rollout alignment (proposal phases)

### Phase 1 (MVP)
- Steps 1–8, with polling-based UI and a limited source allowlist.

### Phase 2
- Add `product_draft_sources` evidence and optional image rehosting to CDN/R2.
- Improve category auto-mapping.

### Phase 3
- Product discovery automation, bulk draft creation, A/B copy generation/localization.

---

## Acceptance Criteria (MVP)
- Draft generation returns within ~1–3 minutes for supported domains.
- Draft includes: title, description, images, and at least one priced variant.
- Admin can edit draft fields and approve.
- Approve creates real product records without manual copy/paste.
- Failed drafts are diagnosable (`error_message` + `raw_extraction` stored or referenced).
- Draft retention: drafts can be TTL-cleaned after approval (e.g. 7–30 days) to keep Turso storage small.

---

## Open Questions (to finalize sequencing and integration)
1. Where will the endpoints + Inngest functions run (Vercel/Next API routes, Go service, etc.)?
2. How do you currently create products (existing resource/dataProvider), and what are the minimum required fields?
3. Which domains should be on the MVP allowlist (3–10 sources)?
