# Feature Proposal: Per-job Chrome DevTools Preflight via `/json/new`

## Problem

The crawler talks to Chrome via Chrome DevTools Protocol (CDP). In long-lived Chrome sessions (especially GUI/VNC), CDP can become partially wedged:
- `/json/version` may still respond
- but normal CDP operations time out when trying to create/use a tab (“Aw, Snap!”, renderer crash, unresponsive target, etc.)

This leads to jobs starting and then failing deeper in the crawl, after we’ve already spent time/resources.

## Goal

Before processing each crawl job, perform a **strong CDP liveness check** by creating a fresh tab/target via:

- `GET http://<host>:<port>/json/new`

If Chrome can create a target, CDP is in a healthier state for doing work.

## Proposed Behavior

### In `crawlworker` (AMQP consumer)

For each job:
1. Validate the message payload (`url`, `event_id`, etc.)
2. Resolve the effective DevTools host and port (existing Docker-aware logic)
3. **Preflight**:
   - call `/json/new` with a short timeout (e.g. `3s`)
   - parse the returned JSON to extract the created `target_id`
   - immediately close it via `/json/close/<target_id>` (best-effort) to avoid leaking tabs
4. Only after preflight succeeds, start the actual crawling work

### Failure semantics (to be finalized)

If the preflight fails (timeout / non-2xx / invalid response):
- return an error from the job handler so the AMQP consumer can apply its configured retry/requeue policy.

Note: the exact impact depends on the consumer’s ack/requeue behavior. This proposal assumes “handler error” leads to retry/requeue.

## Why `/json/new` (and not “refresh”)

“Refresh” requires an existing tab/target to refresh. In a long-lived Chrome:
- there may be many tabs
- some may be crashed
- “the right tab” may not be deterministic

Creating a new target is a simpler, more deterministic check:
- it exercises the DevTools target lifecycle
- it tends to fail when Chrome is wedged even if `/json/version` still works

This aligns with the recommended stronger check mentioned in `docs/chrome_auth_profile_headless_ops.md`.

## Implementation Sketch (Go)

### New package APIs

Add to `internal/pkg/chromedevtools`:

- `NewTarget(ctx, host, port string, timeout time.Duration) (targetID string, err error)`
  - calls `/json/new`
  - parses response `{ "id": "...", ... }`
- `CloseTarget(ctx, host, port, targetID string, timeout time.Duration) error`
  - calls `/json/close/<id>`
- convenience: `PreflightCreateAndCloseTarget(...) error`
  - calls `NewTarget`, then `CloseTarget` (best-effort)

All APIs should reuse existing `VersionURLResolved`/host resolution patterns so Docker + `host.docker.internal` are handled consistently.

### Worker integration point

Add a call near the start of:
- `internal/app/amqp/crawlworker/handler.go` in `(*CrawlHandler).Handle`

Log keys (examples):
- `crawlworker_devtools_preflight_failed` with `event_id`, `host`, `port`, `err`
- `crawlworker_devtools_preflight_ok` with `event_id`, `host`, `port`, `target_id` (optional)

## Configuration (optional)

This can start as hardcoded constants (conservative defaults), but if needed later:
- `CHROME_DEVTOOLS_PREFLIGHT_ENABLED` (default: true)
- `CHROME_DEVTOOLS_PREFLIGHT_TIMEOUT_MS` (default: 3000)

## Tests

Add unit tests under `internal/pkg/chromedevtools` with `httptest`:
- `/json/new` returns an id → ok
- `/json/new` returns non-JSON / missing id → error
- `/json/new` non-2xx / timeout → error
- `/json/close/<id>` is called best-effort (close failure should not override new-target success if we choose best-effort close)

## Rollout / Ops Notes

- This change is “preflight only”; it does not change crawl logic.
- If Chrome is down/wedged, jobs will fail faster and (depending on consumer policy) retry faster.
- If preflight increases load on Chrome, adjust timeouts or disable preflight via config.

