# Docker `docker-once` Redesign Plan (Parity With `dev-once`)

## Goal

Provide a Docker-based, one-shot crawl command that is **behaviorally identical** to the host `dev-once` loop, while keeping the existing “runner-in-Docker, Chrome-on-host” model.

Target UX:

```bash
make docker-once tool=codex  url=https://shopee.tw/...
make docker-once tool=gemini url=https://shopee.tw/...
```

Outputs still land in `./out/` on the host.

## Current Problems (Why `docker-once` Is “Messed Up”)

1) **CLI flag mismatch (hard failure)**

- `docker-compose.yml` currently runs the container command with `--tool "$CRAWL_TOOL"`.
- The binary executed in the container is `/app/runner` (built from `cmd/runner`).
- `cmd/runner` does **not** define a `--tool` flag today, so Docker runs fail immediately.

2) **Parity gaps (soft failures / confusing behavior)**

- Docker runner service does not explicitly mirror key env defaults used by host tooling (e.g. DevTools host/port and model envs).
- `dev-once` and Docker can drift in tool/model selection unless the operator remembers to set multiple env vars.

## Redesign Principles

- **Single source of truth for options**: the Docker runner should accept the same conceptual inputs as `dev-once` (`url`, `tool`, `model`, `outDir`, optional `promptFile`).
- **Backward compatible** where practical: keep old runner flags working (especially `--codex-cmd`) while adding new parity flags.
- **No architecture migration**: this is a tooling/CLI + compose wiring fix; no FX/router changes required.

## Proposed Interface (Contract)

### Makefile: `docker-once`

Same interface and validation as `dev-once`:

- Required: `url=<product_url>`
- Optional: `tool=codex|gemini` (default `codex`)

Implementation detail:

- `make docker-once` runs the crawl inside Docker via `docker compose run --rm --build runner ...`.
- The out dir inside Docker should be `/out` (bind-mounted to host `./out`), so results persist.

### Container command (runner)

The Docker run should explicitly pass flags to `/app/runner`:

- `--url <url>` (required)
- `--tool codex|gemini` (new; parity with `dev-once`)
- `--out-dir /out` (persist)
- optional: `--model <model>` (new; parity with `dev-once`)
- optional: `--prompt-file <path>` (already exists)

## Required Code Changes (Updated Approach)

Instead of running `/app/runner` in Docker, run the same “devtool once” command that the host uses, as a compiled binary inside the image.

### 1) Add `devtool` binary to the Docker image

Update `Dockerfile` to build and copy `./cmd/devtool` into the runtime image as `/app/devtool`.

### 2) Switch `docker-once` to execute `/app/devtool once`

Update `Makefile` so `make docker-once ...` runs:

```bash
docker compose run --rm --build runner /app/devtool once --tool <tool> --url <url> --out-dir /out
```

### 3) Update `docker-compose.yml` runner service command template

Ensure the `runner` service includes the same relevant env vars as `server`:

- `CHROME_DEBUG_HOST` (default `host.docker.internal`)
- `CHROME_DEBUG_PORT` (default `9222`)
- `CRAWL_TOOL` (default `codex`)
- `CODEX_MODEL`, `GEMINI_MODEL` (optional defaults)
- `CODEX_SKIP_GIT_REPO_CHECK` (keep existing default)

Then update the runner service command to use the corrected flags:

-- Use `/app/devtool once --url "$TARGET_URL" --tool "$CRAWL_TOOL" --out-dir /out`

### 4) Makefile `docker-once` redesign

Replace the current `docker compose run ... -e TARGET_URL=... -e CRAWL_TOOL=... runner` approach with an explicit command invocation so the contract is clear and stable:

- Always pass `--url` and `--tool`.
- Force output directory to `/out`.
- Keep `docker-doctor` as a prerequisite.

## Validation Plan

1) Compile & unit tests:

```bash
go test ./...
go run ./cmd/runner --help
```

2) Manual parity check:

```bash
make dev-chrome
make docker-once tool=codex  url=https://shopee.tw/...
make docker-once tool=gemini url=https://shopee.tw/...
```

Confirm:

- Docker run does not error on unknown flags.
- Result JSON is written under `./out/` on the host.
- The runner can connect to host Chrome DevTools (`host.docker.internal:9222`).

## Expected Outcome

- `docker-once` becomes a predictable “parity check” command: same knobs as `dev-once`, same output location, just executed inside Docker.
- The compose runner command becomes correct (no nonexistent flags) and easier to debug because `/app/runner` is invoked explicitly with flags.
