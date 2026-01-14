# Refactor Plan: Reusable Chrome DevTools Reachability Check

## Background
- `make dev-doctor` runs `go run ./cmd/devtool doctor`.
- The current doctor logic performs `GET http://127.0.0.1:<port>/json/version` with a short timeout, requires `2xx`, and requires a non-empty response body.
- `docker-doctor` duplicates most of the same reachability check.
- Goal: reuse the same check inside `internal/app/inngest/crawl/crawl.go` as a preflight step, so Inngest jobs fail fast when DevTools is not available.

## Goals
- Make the DevTools reachability check reusable from both CLI (`cmd/devtool`) and server/inngest code (`internal/...`).
- Keep behavior consistent with today’s `dev-doctor` check (timeout, status handling, non-empty response requirement).
- Avoid changes to `config/config.go` structure/spec.

## Non-goals
- Do not change how Chrome is launched (`cmd/devtool chrome`) or how the crawler uses DevTools beyond adding a preflight.
- Do not introduce global singletons; keep everything as plain functions called from the appropriate layers.

## Proposed Changes

### 1) Extract helper package
- Add `internal/pkg/chromedevtools` with a small API:
  - `func VersionURL(host, port string) string` (builds `http://<host>:<port>/json/version`)
  - `func CheckReachable(ctx context.Context, url string, timeout time.Duration) ([]byte, error)`
- `CheckReachable` behavior:
  - Use `http.Client{Timeout: timeout}` (or `ctx` + client timeout; whichever is clearer/consistent).
  - `GET` the URL.
  - Error if request fails, status is not `2xx`, or body is empty after `bytes.TrimSpace`.
  - Read and cap body size (e.g. `io.LimitReader` to 32KB), mirroring current doctor behavior.

### 2) Refactor CLI doctor commands to call helper
- Update `cmd/devtool/cmd/doctor.go` to call `chromedevtools.CheckReachable(...)`.
  - Keep current CLI UX and error messages (port flag, printing “Checking: …”, and success message).
- Update `cmd/devtool/cmd/docker_doctor.go` to reuse the same helper for the DevTools portion.
- Remove `bytesTrimSpace` from `cmd/devtool/cmd/util.go` if it becomes unused (or leave it if still referenced elsewhere).

### 3) Add Inngest crawl preflight step
- In `internal/app/inngest/crawl/crawl.go`, add a new step before `run-crawler`, e.g. `check-devtools`.
- Determine port without changing config spec:
  - Use `CHROME_DEBUG_PORT` env var with default `9222` (same as existing CLI), and host `127.0.0.1`.
- If DevTools is not reachable:
  - Log a clear message with the URL being checked.
  - Return `inngestgo.NoRetryError(err)` so it fails fast (rather than retrying indefinitely).

### 4) Tests
- Add unit tests for `internal/pkg/chromedevtools` using `httptest.Server`:
  - `200` with non-empty body -> success
  - `204` or whitespace body -> error
  - `500` -> error
  - timeout / context cancellation -> error

## Acceptance Criteria
- `go run ./cmd/devtool doctor` still performs the same check and succeeds/fails as before.
- `go run ./cmd/devtool docker-doctor` still validates DevTools reachability (using the shared helper).
- Inngest crawl (`internal/app/inngest/crawl/crawl.go`) fails fast with a clear error when DevTools is unreachable.
- New helper has unit tests and `go test ./...` passes.

