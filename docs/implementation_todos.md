# Implementation TODOs (Dev-friendly + Deployable)

This checklist assumes the architecture in `docs/shopee_crawler_plan.md` (host Chrome + DevTools + Docker runner + Codex CLI + `chrome-devtools-mcp`).

## A) Repository shape / ergonomics

- [x] Decide the canonical workspace layout (repo root).
- [x] Add a top-level `Makefile` (or `justfile`) with:
  - `make dev-chrome` (start Chrome)
  - `make dev-doctor` (check `:9222`)
  - `make dev-once URL=...` (one-shot crawl)
  - `make docker-once TARGET_URL=...` (parity check)
- [x] Standardize env var names and document them:
  - `CHROME_DEBUG_PORT`, `CHROME_PROFILE_DIR`, `TARGET_URL`

## B) Codex + MCP configuration

- [x] Define the “official” Codex MCP server config for:
  - local dev (browser URL: `http://127.0.0.1:9222`)
  - Docker (browser URL: `http://host.docker.internal:9222`)
- [x] Install `codex` in the Docker image (via `npm install -g`, configurable with `CODEX_NPM_PKG` build arg).
- [ ] Add a `cmd/devtool` subcommand (optional) to print/validate Codex MCP config.

## C) Crawler prompt + extraction contract (stability first)

- [ ] Finalize `config/schema.product.json`:
  - exact types for `price` (number vs string)
  - required fields when `status="ok"`
  - optional `raw` / `debug` fields policy (if ever allowed)
- [ ] Harden `config/prompt.product.txt`:
  - explicit “wait until” conditions
  - explicit selectors strategy (fallbacks)
  - stricter “ONLY JSON” instruction + no markdown
  - clear detection rules for login/verify/CAPTCHA walls

## D) Go runner (core)

- [ ] Add a package structure (recommended):
  - `internal/prompt` (template substitution)
  - `internal/codex` (invoke `codex exec`, timeouts, capture stderr)
  - `internal/result` (schema validation, normalization)
  - `internal/io` (out file naming, atomic writes)
- [ ] Add timeouts:
  - per-run timeout (kill `codex exec` if stuck)
  - navigation wait budget (encoded in prompt + runner timeout)
- [ ] Add a single-run lock to prevent overlap (PID file or file lock in `/out`).
- [ ] Implement URL sources:
  - `--url` (single)
  - `--urls-file` (iterate `urls.txt`, ignore blanks/comments)
  - de-dup + state (skip recently crawled)
- [ ] Implement runner exit codes:
  - `0` when it produced output(s)
  - distinct non-zero when config is invalid (optional)
- [ ] Validate output against JSON schema:
  - pick a Go JSON Schema validator
  - fail closed when invalid JSON is returned

## E) Scheduler / operational loop

- [ ] Decide scheduling model:
  - Cron in container
  - Internal ticker loop
  - External orchestrator (k8s CronJob)
- [ ] Implement backoff policy:
  - on `needs_manual`, stop and retry with exponential backoff
  - emit a clear “manual action needed” log line
- [ ] Add structured logs (JSON logs recommended) with:
  - url, status, duration, error

## F) Docker deployment hardening

- [ ] Create a production Dockerfile that:
  - builds Go runner
  - includes Node/NPM for `npx chrome-devtools-mcp@latest`
  - includes (or expects) `codex`
  - runs as a non-root user (where feasible)
- [ ] Pin versions:
  - base image tags
  - Node + `chrome-devtools-mcp` version
- [ ] Add health checks:
  - reachability of `host.docker.internal:9222` (Docker Desktop)
  - optional “can list pages” via MCP

## G) Debugging workflow (DX)

- [ ] Add a “debug mode” runner flag:
  - writes raw Codex stdout/stderr to `out/debug/`
  - optionally writes the exact prompt used
- [ ] Add a “fixture mode” for prompt development:
  - load a saved HTML snapshot and test parsing logic (if you implement parsing outside the browser)
- [ ] Add a troubleshooting doc section for common Shopee UI changes/selectors.
