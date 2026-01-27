# Feature Plan: Distributed Crawling Pipeline (Turso + RabbitMQ + Go Workers)

## 1. Summary

Build a distributed crawling system where:

1) **Feed Ingestor Worker** scans Threads feeds for a set of monitored accounts, extracts Shopee/Taobao links, and **creates crawl jobs** in **Turso (libSQL / SQLite)**.

2) A **Dispatcher** publishes lightweight messages (usually just `job_id`) to **RabbitMQ**.

3) Multiple **Crawler Workers** (including home PCs behind NAT with dynamic IPs) consume jobs from RabbitMQ, **claim** them in Turso to prevent duplicates, execute crawling, and write results back to Turso (and optionally object storage).

**Key point:** workers do **not** need public IPs. They maintain outbound TCP connections to the RabbitMQ broker and receive messages over those connections.

---

## 2. Goals

- Reliable job distribution to many worker nodes (heterogeneous machines, dynamic IPs).
- Central, auditable job store and job state machine in Turso.
- At-least-once delivery with **idempotent job execution** (safe to retry).
- Clear retry / dead-letter handling for failing jobs.
- Simple Go integration and operational visibility (metrics + logs).

## 3. Non-Goals (initially)

- Exactly-once processing end-to-end (we’ll do at-least-once + idempotency).
- Full UI for ops (can be added later).
- Advanced scheduling and prioritization beyond basic queue separation / routing keys.

---

## 4. High-Level Architecture

```mermaid
flowchart LR
  A[Threads Feed Ingestor] -->|INSERT job| B[(Turso: crawl_jobs)]
  A -->|Publish job_id| C[(RabbitMQ)]
  C --> D1[Crawler Worker #1]
  C --> D2[Crawler Worker #2]
  C --> D3[Crawler Worker #N (home PC)]
  D1 -->|Claim + Update status| B
  D2 -->|Claim + Update status| B
  D3 -->|Claim + Update status| B
  D1 -->|Write result| B
  D2 -->|Write result| B
  D3 -->|Write result| B
```

---

## 5. Components

### 5.1 Feed Ingestor Worker (Threads Scanner)

Responsibilities:
- Poll Threads feeds for configured accounts.
- Extract candidate URLs (Shopee, Taobao).
- Normalize URLs (strip tracking params, canonicalize if needed).
- Deduplicate at source (avoid repeated creation of same job).
- Insert new job rows into Turso.

Suggested output:
- Create a `crawl_jobs` row with status `NEW`.
- Publish a message to RabbitMQ with `{job_id}` or `{job_id, type, priority}`.

### 5.2 Job Store (Turso / libSQL)

Turso is the **source of truth**:
- Job lifecycle state and audit trail
- Lease/lock info to avoid duplicates
- Retry counters and error history
- Crawled payloads + extracted product data (or pointers to blob storage)

Optional performance upgrade later:
- Embedded replicas for read-heavy nodes (if you want lower latency reads).

### 5.3 Dispatcher (optional if Ingestor publishes directly)

Two options:
- **Option A (simplest):** Ingestor publishes directly to RabbitMQ after inserting the job.
- **Option B:** Ingestor only writes DB; a Dispatcher polls `NEW` jobs and publishes to RabbitMQ, marking them `QUEUED`.

Option A is usually enough if you handle “publish failed” recovery (see §10.3).

### 5.4 Crawler Workers (many nodes)

Responsibilities:
- Consume from RabbitMQ (manual ack).
- Fetch job details from Turso.
- Claim the job (atomic DB update) to prevent duplicate active processing.
- Crawl target URL.
- Persist results and final status to Turso.
- Ack message only after successful completion (or after deciding retry/dead-letter).

### 5.5 Observability

- Structured logs: `job_id`, `worker_id`, `attempt`, `queue`, `status`, latency.
- Metrics: jobs/sec, success rate, retry rate, time-to-crawl, DLQ counts.
- Dashboard: RabbitMQ management UI, plus app metrics (Prometheus/OpenTelemetry).

---

## 6. Data Model (Turso)

### 6.1 `crawl_jobs` table

Recommended columns:

- `id` (TEXT/UUID, primary key)
- `source` (TEXT) — e.g. `threads`
- `source_account` (TEXT)
- `source_post_id` (TEXT) — if available
- `url` (TEXT) — original URL
- `url_norm` (TEXT) — normalized URL (unique)
- `platform` (TEXT) — `shopee` | `taobao` | `other`
- `status` (TEXT) — state machine (see below)
- `priority` (INTEGER) — default 0
- `attempt` (INTEGER) — starts at 0
- `max_attempts` (INTEGER) — e.g. 5
- `locked_by` (TEXT) — worker id
- `locked_at` (DATETIME)
- `lock_expires_at` (DATETIME)
- `last_error` (TEXT)
- `created_at` (DATETIME)
- `updated_at` (DATETIME)
- `started_at` (DATETIME)
- `finished_at` (DATETIME)
- `result_json` (TEXT/JSON) — product data, crawl metadata
- `content_hash` (TEXT) — optional: idempotency and change tracking

Suggested indexes:
- `UNIQUE(url_norm)`
- `INDEX(status, priority, created_at)`
- `INDEX(lock_expires_at)`
- `INDEX(platform, status)`

### 6.2 State Machine

Minimum viable states:

- `NEW` — created in DB
- `QUEUED` — message published to RabbitMQ (optional)
- `RUNNING` — claimed by a worker
- `SUCCEEDED` — done
- `FAILED_RETRYABLE` — failed but eligible for retry
- `FAILED_PERMANENT` — not retryable
- `DEAD` — moved to DLQ / requires manual intervention

Keep the state machine simple at first; add more granular states later if needed.

---

## 7. RabbitMQ Topology

### 7.1 Exchanges & Queues

Recommended:

- Exchange: `crawl.jobs` (type: `direct` or `topic`)
- Queues:
  - `crawl.shopee`
  - `crawl.taobao`
  - `crawl.other` (optional)
- Routing keys:
  - `shopee`, `taobao`, `other`

Bindings:
- `crawl.jobs` + `shopee` -> `crawl.shopee`, etc.

### 7.2 Manual Ack + Prefetch

- Use **manual acknowledgements** (autoAck=false).
- Set a **prefetch count** per worker to control concurrency and memory usage (e.g. 1–5).
- This ensures workers won’t get flooded with unacknowledged deliveries.

### 7.3 Dead Letter Exchange (DLX)

Configure each queue with:
- `x-dead-letter-exchange: crawl.dlx`
- `x-dead-letter-routing-key: <queue-name or platform>`

DLQ examples:
- `crawl.dlq.shopee`
- `crawl.dlq.taobao`

When to dead-letter:
- Exceeded max attempts
- Non-retryable errors (e.g. 404 / removed listing)
- Invalid payload / cannot parse

### 7.4 Retry Strategy

Two practical patterns:

**Pattern A: DB-driven retry (simplest)**
- Worker sets `FAILED_RETRYABLE`, increments `attempt`, stores `next_run_at`.
- A Retry Scheduler republishes due jobs to the main exchange.

**Pattern B: TTL + DLX delay queue (RabbitMQ pattern)**
- Publish to a delay queue with `x-message-ttl = backoff_ms` and DLX back to main queue.
- Useful if you want retries without polling.

Start with Pattern A unless you already operate a standard delay-queue setup.

---

## 8. Worker Claiming & Idempotency (Critical)

Because MQ delivery is at-least-once, implement **idempotency**:

### 8.1 Claim algorithm

When worker receives message `{job_id}`:

1) Load job from Turso.
2) Attempt claim (atomic):
   - Only claim if status is `NEW` or `QUEUED` or lock expired.
   - Update: `status='RUNNING'`, `locked_by`, `locked_at`, `lock_expires_at = now + lease`.
3) If claim succeeds: crawl.
4) If claim fails: someone else is working → ack message (or requeue depending on policy).

### 8.2 Lease expiration

- Set `lock_expires_at` (e.g. now + 10 minutes).
- If worker crashes, another worker can re-claim after expiry.
- Update `lock_expires_at` periodically for long crawls (heartbeat), or set a sufficiently long lease.

### 8.3 “Publish succeeded but DB update failed” & vice versa

- Prefer order: **DB insert first, then publish**.
- If publish fails: keep status `NEW` and retry publish later (dispatcher/scheduler).
- If publish succeeds but DB write fails (rare if you do DB first): treat as poison; log and alert.

---

## 9. Security Model

- Do **not** expose Redis/RabbitMQ ports openly to the public internet.
- Prefer one of:
  - Broker behind a VPN (WireGuard/Tailscale) so home workers join the private network.
  - Broker with TLS + strong auth, allowlisted IP ranges (still tricky with dynamic home IPs).
- RabbitMQ should be configured with TLS for AMQPS when used over the Internet.

---

## 10. Implementation Phases

### Phase 0 — Proof of Concept (1 day)

- Turso table `crawl_jobs`.
- Simple producer: create job, publish `job_id`.
- Single worker: consume, claim, print job URL, mark `SUCCEEDED`.

Deliverable:
- End-to-end job flows with manual ack.

### Phase 1 — Core MVP (2–5 days)

- Feed ingestor extracts URLs and creates jobs with dedupe.
- Multiple workers with prefetch=1–3.
- Basic retry counter and error logging.
- Job result persistence.

Deliverable:
- Can scale to N worker nodes.

### Phase 2 — Reliability & Operations (3–7 days)

- DLX + DLQ routing.
- Retry scheduler / backoff.
- Reconnection logic for RabbitMQ channel/connection.
- Metrics and dashboard.

Deliverable:
- Handles worker crashes and transient failures cleanly.

### Phase 3 — Performance & Cost Optimizations (later)

- Embedded replicas for read-heavy workloads (optional).
- Workload-based routing (priority queues).
- Rate limiting per platform.
- Distributed tracing.

---

## 11. Acceptance Criteria

- A new Threads feed URL produces exactly one `crawl_jobs` row (deduped by `url_norm`).
- A published job is processed by any available worker without public inbound access.
- If a worker crashes mid-job, the job is re-processed after lease expiry.
- Jobs that fail repeatedly end up in DLQ / `DEAD` state with clear error metadata.
- Operator can requeue a DLQ job (manual action) and it works.

---

## 12. Notes for Go Implementation (Pointers, not code)

- Use RabbitMQ Go client maintained by RabbitMQ core team.
- Use manual ack + QoS prefetch.
- Consider publisher confirms for producer robustness.
- Turso Go SDK: use `go-libsql` (CGO) if you want embedded replicas; otherwise non-CGO client works but lacks embedded replicas.

---

## 13. References (readable sources)

```text
RabbitMQ Go client (amqp091-go): https://github.com/rabbitmq/amqp091-go
Go package docs (amqp091-go): https://pkg.go.dev/github.com/rabbitmq/amqp091-go

RabbitMQ consumer prefetch (basic.qos): https://www.rabbitmq.com/docs/consumer-prefetch
RabbitMQ consumer acknowledgements: https://www.rabbitmq.com/docs/consumers
RabbitMQ confirms (acks/prefetch mention): https://www.rabbitmq.com/docs/confirms
RabbitMQ dead-letter exchanges: https://www.rabbitmq.com/docs/dlx

Turso embedded replicas intro: https://docs.turso.tech/features/embedded-replicas/introduction
Turso Go SDK reference: https://docs.turso.tech/sdk/go/reference
Turso Go quickstart: https://docs.turso.tech/sdk/go/quickstart
```
