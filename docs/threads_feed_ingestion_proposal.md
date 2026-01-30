# Threads Feed Ingestion (Daily Cron) — Proposal

## 1) Summary

We want to “subscribe” to a set of Threads profile pages (e.g. `https://www.threads.com/@nanaken237611`), crawl the newest post per account once per day, extract:

- Hook line / caption text (and optionally an LLM-normalized summary)
- Media (image/video URLs; optionally downloaded artifacts)
- Outbound product links (Shopee mostly; some Taobao)

…persist the structured result into Turso SQLite (libSQL), and then enqueue the product URLs as AMQP jobs for existing downstream crawling via `internal/app/amqp/crawlworker/handler.go`.

This doc proposes a minimal, maintainable design that fits the current repo conventions (FX, optional SQLite, RabbitMQ consumer already exists).

## 2) Goals / Non-goals

**Goals**
- Daily ingestion: newest post per subscribed account.
- Deterministic extraction where possible; LLM only where helpful (hook line cleanup, edge cases).
- Idempotent storage + idempotent AMQP publish (avoid duplicate product crawl jobs).
- Designed to tolerate SQLite disabled (skip persist or fail fast; choose behavior explicitly).

**Non-goals (for v1)**
- Full historical backfill of all posts.
- Perfect extraction for every post type (quotes, reposts, carousels, etc.).
- Guaranteed access when Threads requires login / blocks automation.

## 3) Key Constraints & Risks

### 3.1 Threads “API” availability
Assume **no reliable open API** to fetch arbitrary third-party profiles/posts. Any official API is likely limited to your own account context. Therefore the baseline approach is **browser automation + DOM extraction**.

### 3.2 Fragility / anti-bot
Threads is a dynamic web app; selectors and markup may change. You should expect:
- occasional failures due to UI changes
- rate limiting / bot detection
- some profiles requiring login to view posts

Mitigations are included below (low crawl rate, stable selectors, fallback parsing, evidence capture).

### 3.3 Compliance / ToS
Scraping may violate Threads ToS. Treat this as an explicit product decision and implement:
- a “kill switch” to disable ingestion
- conservative request rate
- strong observability and quick rollback

## 4) Proposed Architecture

### 4.1 Components

1) **Cron binary**: `cmd/threads-ingest`
   - Reads subscription list from config (viper) or from SQLite table.
   - For each account:
     - navigates to profile page via Chrome DevTools
     - finds the newest post (post URL + timestamp)
     - extracts caption text + media + outbound links
     - upserts into Turso SQLite
     - emits AMQP jobs for each product link

2) **Threads extraction package**: `internal/app/threads/...`
   - `crawler` (DevTools navigation + DOM query)
   - `extractor` (parse into canonical struct)
   - no global singletons; constructed via FX when run inside long-lived worker/server; but **cron can be a small standalone FX app**.

3) **SQLite DAO**: `internal/app/threads/dao`
   - Owns schema access and upserts.
   - Uses injected `db.Conn name:"sqlite" optional:"true"`.

4) **AMQP publisher** (reusing existing message shape)
   - Publishes `crawler/url.requested` jobs (same `CrawlRequestedEnvelope` shape used by the consumer).
   - Uses stable `event_id` for dedupe.

### 4.2 Execution Model

- Run daily (e.g. 02:00 local time) via:
  - system `crontab`, or
  - container scheduler (recommended for deployment), or
  - GitHub Actions / cloud cron (if the crawler can reach Chrome DevTools)

**Important:** DevTools must be reachable from the cron environment. The repo already has patterns around `cfg.Chrome.DebugHost/DebugPort`.

## 5) Data Model (SQLite / Turso)

### 5.1 Tables (proposed)

**`threads_subscriptions`**
- `account_handle` (PK) — e.g. `nanaken237611`
- `profile_url` — canonical profile url
- `enabled` (bool)
- `notes` (text, optional)
- `created_at`, `updated_at`
- `last_seen_post_url` (text, nullable)
- `last_seen_post_published_at` (datetime, nullable)

**`threads_posts`**
- `post_url` (PK)
- `account_handle` (FK-ish; keep simple if you don’t enforce FK)
- `published_at` (datetime, nullable if unknown)
- `caption_text` (text, nullable)
- `hook_line` (text, nullable; possibly LLM-normalized)
- `raw_json` (text, nullable) — store extractor output / evidence pointers
- `created_at`, `updated_at`

**`threads_post_media`**
- `id` (PK)
- `post_url`
- `media_type` (`image` | `video`)
- `media_url` (text)
- `thumb_url` (text, nullable)
- `local_path` (text, nullable, only if you choose to download)
- unique `(post_url, media_url)`

**`threads_post_links`**
- `id` (PK)
- `post_url`
- `url` (text)
- `domain` (text, extracted)
- `kind` (`product` | `other`) — v1: classify by domain heuristics + optional LLM assist
- unique `(post_url, url)`

**`threads_ingest_runs`**
- `id` (PK)
- `started_at`, `finished_at`
- `status` (`ok` | `partial` | `failed`)
- `error` (text, nullable)
- `stats_json` (text, nullable)

### 5.2 Dedupe & Idempotency

Two independent dedupe layers:
- **Post dedupe**: `threads_posts.post_url` primary key.
- **AMQP job dedupe**: deterministic `event_id`:
  - `event_id = sha256("threads:" + post_url + ":" + product_url)`
  - This makes reruns safe (consumer already requires `event_id`).

## 6) Extraction Strategy (Chrome DevTools)

### 6.1 Steps per account (happy path)
1) Navigate to `https://www.threads.com/@{handle}`.
2) Wait for “posts grid / timeline” element to appear.
3) Identify the newest post:
   - Prefer: first post card with a permalink anchor.
4) Open the post permalink in the same tab.
5) Extract:
   - caption text: the post text container
   - media: image/video elements (collect src URLs)
   - outbound links: anchor tags in the caption and link preview cards
6) Normalize:
   - trim, de-duplicate URLs, resolve redirect URLs if possible
   - classify product link domains (Shopee/Taobao)

### 6.2 Selector robustness
Avoid brittle CSS class selectors. Prefer:
- aria labels / roles
- link URL patterns (e.g. anchors containing `/@handle/post/` if such exists)
- “closest anchor” heuristics

### 6.3 Evidence capture (recommended)
On each account run, store at least one of:
- HTML snapshot (outerHTML of main container)
- screenshot path
- extracted JSON with the raw nodes/attributes used

This makes UI breakages debuggable.

## 7) LLM Use (Gemini/Codex)

LLM should be optional and bounded:

**Recommended v1 LLM usage**
- Derive `hook_line`:
  - input: caption text (and optionally alt text)
  - output: short “hook line” (<= 120 chars) + language detection + keywords
- Link classification:
  - only if heuristics are insufficient

**Hard rules**
- Don’t rely on LLM to “find” URLs that aren’t already extracted deterministically.
- Enforce JSON schema output (the repo already has notes/plans around JSON enforcement).

## 8) AMQP Integration

### 8.1 Publish shape
Reuse `internal/app/amqp/crawlworker/message.go` envelope:
- `event_name`: `crawler/url.requested` (or omit; consumer accepts empty or that exact value)
- `event_id`: deterministic hash as above
- `data.url`: product URL
- `data.out_dir`: optional (defaults to `out` in handler)

### 8.2 Backpressure / limits
Daily ingestion can still produce many URLs. Add:
- per-run cap (e.g. max N URLs enqueued)
- per-account cap (e.g. only newest post; max M product links)

## 9) Configuration

### 9.1 Minimal config (proposed)
- `THREADS_INGEST_ENABLED` (bool; kill switch)
- `THREADS_SUBSCRIPTIONS` (CSV handles or JSON array)
- existing Chrome devtools config (`cfg.Chrome.*`)
- existing SQLite config (`TURSO_SQLITE_*`)
- existing RabbitMQ config

### 9.2 Where to store subscriptions
Two options:
- **Config-first** (fastest): `THREADS_SUBSCRIPTIONS=nanaken237611,otherhandle`
- **DB-first** (more flexible): store handles in `threads_subscriptions` and add a small CLI to manage them

Recommend: **config-first for v1**, then add DB subscription management later.

## 10) Operational Notes

- Run time: keep slow and steady (human-like). Threads is not a feed API.
- Concurrency: default sequential; optionally small concurrency (2–3) if stable.
- Retries: on transient failures, retry a small number of times with jitter.
- Observability: log per account:
  - newest post URL
  - number of links/media extracted
  - sqlite persist status
  - enqueued count

## 11) Phased Implementation Plan

**Phase 0 — Spike (1–2 days)**
- Implement minimal “get newest post permalink + caption + outbound links” for 1 hard-coded account using DevTools.
- Log structured JSON, no DB writes.

**Phase 1 — Persistence + AMQP**
- Add tables + goose migrations.
- Implement DAO upserts.
- Publish AMQP jobs per product link.

**Phase 2 — Robustness**
- Evidence capture + improved selector strategy.
- “last seen” tracking to avoid reprocessing the same post.

**Phase 3 — LLM enhancements**
- Hook line normalization and multilingual support.
- Better link classification.

## 12) Open Questions (to confirm before coding)

1) Should ingestion **fail** when SQLite is disabled, or **skip persist** (log and still enqueue AMQP)?
2) Do we need to **download media** (for permanence) or just store URLs?
3) Are we allowed to authenticate (login) to Threads in the crawler environment? If yes, we need a secure cookie/session strategy.
4) How many accounts are expected (10 vs 1,000)? This affects crawl rate and infra design.

