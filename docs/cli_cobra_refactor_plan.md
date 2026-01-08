# CLI Refactor Plan (Cobra)

This document proposes how to refactor the current Go CLI(s) to use `spf13/cobra` for a clearer command structure, consistent help/errors, easier testing, and cleaner long-term maintenance.

This repo’s crawler architecture and constraints remain the source of truth in `docs/shopee_crawler_plan.md` (host Chrome via DevTools + Docker runner + Codex CLI + `chrome-devtools-mcp`, Chrome 136+ `--user-data-dir`, Docker `host.docker.internal`, CAPTCHA can require manual intervention).

## Goals

- Make the CLI **more structural/organized** (clear command tree, consistent flags, consistent UX).
- Improve **maintainability** (smaller command files, shared helpers, fewer duplicated utilities).
- Improve **readability** (self-documenting `Use/Short/Long/Example`, automatic help).
- Preserve the existing **crawler output contract** (JSON with `status: ok | needs_manual | error`).
- Keep dev & deploy parity (same config files, no “dev-only” logic paths).

## Current State (What We Have)

There are two binaries under `cmd/`:

- `cmd/devtool`:
  - Manual command dispatch via `os.Args[1]` switch.
  - Per-command parsing via `flag.NewFlagSet`.
  - Commands: `chrome`, `doctor`, `docker-doctor`, `once`.
- `cmd/runner`:
  - Single command using global `flag` parsing.
  - Calls `internal/runner.RunOnce` and prints output path.

Observed pain points:

- Manual dispatch and usage text duplication (`cmd/devtool/main.go`).
- Inconsistent help behavior (flag parse errors don’t naturally show help; output is discarded for FlagSets).
- Duplicated env helpers (`getenvDefault/getenvBool`) across binaries.
- Harder to test CLI behavior (manual parsing and `os.Exit` patterns).
- Harder to evolve CLI surface area (new flags/commands add boilerplate).

## Recommended Direction

### Option A (Recommended): Keep two binaries, introduce Cobra in each

This is the lowest-risk refactor. It preserves the current Makefile/Dockerfile mental model and avoids renaming binaries.

- `cmd/devtool` becomes a Cobra root command with subcommands:
  - `devtool chrome`
  - `devtool doctor`
  - `devtool docker-doctor`
  - `devtool once`
- `cmd/runner` becomes a Cobra root command `runner` (even if it stays single-purpose today).

Pros:
- Minimal impact on deployment wiring (`docker-compose.yml` continues to run `/app/runner ...`).
- Easy incremental migration (convert devtool first, then runner).

Cons:
- Still two entrypoints (acceptable, but slightly less cohesive).

### Option B (Later): Merge into a single binary with subcommands

Example shape:

- `miner dev chrome|doctor|docker-doctor`
- `miner crawl once`
- (future) `miner crawl batch` / `miner schedule`

Pros:
- One place for shared flags/env/config.
- A more coherent UX.

Cons:
- Requires updating Makefile targets, Docker invocation, and user muscle memory.

## Proposed Command Tree (Option A)

### `devtool`

- `devtool chrome`
  - Flags:
    - `--port` (default from `CHROME_DEBUG_PORT`, else `9222`)
    - `--profile-dir` (default from `CHROME_PROFILE_DIR`, else `$HOME/chrome-mcp-profiles/shopee`)
  - Behavior:
    - Ensure profile dir exists.
    - Launch Chrome with `--remote-debugging-port` and non-default `--user-data-dir` (per `docs/shopee_crawler_plan.md`).

- `devtool doctor`
  - Flags:
    - `--port` (default `CHROME_DEBUG_PORT` / `9222`)
  - Behavior:
    - `GET http://127.0.0.1:<port>/json/version` with short timeout, error if not reachable.

- `devtool docker-doctor`
  - Flags:
    - `--port` (host Chrome DevTools port)
    - `--auth-file` (default `codex/.codex/auth.json`)
  - Behavior:
    - Confirm host Chrome DevTools reachable.
    - Confirm Codex auth JSON exists and is valid (for Docker runs).

- `devtool once`
  - Flags:
    - `--url` (required)
    - `--prompt-file` (default `config/prompt.product.txt`)
    - `--out-dir` (default `out`)
  - Env:
    - `CODEX_CMD` default `codex`
    - `CODEX_SKIP_GIT_REPO_CHECK` forwarded to `internal/runner.Options`
  - Behavior:
    - Call `internal/runner.RunOnce`, print output path.
    - Preserve current semantics: if output file is written, treat crawl errors as non-fatal for CLI exit.

### `runner`

- `runner` root
  - Flags (current):
    - `--url` (required today)
    - `--prompt-file`, `--out-dir`, `--codex-cmd`
  - Future-ready flags (non-breaking, optional to add later):
    - `--urls-file`
    - `--timeout`
    - `--debug-dir` (persist raw stdout/stderr)

## Code Organization (Cobra Layout)

Introduce a conventional Cobra structure:

- `cmd/devtool/main.go`:
  - Minimal: calls `cmd.Execute()` and handles error exit code.
- `cmd/devtool/cmd/root.go`, `cmd/devtool/cmd/chrome.go`, etc.
- `cmd/runner/main.go`:
  - Minimal: calls `cmd.Execute()`.
- `cmd/runner/cmd/root.go`

Shared logic and helpers:

- Keep crawl behavior in `internal/runner` (already good separation).
- Move duplicated env helpers to a small internal package, e.g.:
  - `internal/envutil` (`GetStringDefault`, `GetBoolDefault`)
- (Optional) move “devtool” helpers to `internal/devtool` if command files get too large.

## Migration Steps

1. Add Cobra dependency:
   - `go get github.com/spf13/cobra@latest`
   - `go mod tidy`
   - Note: this requires network access in the environment.
2. Refactor `cmd/devtool`:
   - Replace `os.Args` switch + `usage()` with Cobra root + subcommands.
   - Preserve command names and flags to avoid breaking Make targets.
3. Refactor `cmd/runner`:
   - Convert to Cobra root; keep flag names identical (`-url`, `-prompt-file`, `-out-dir`, `-codex-cmd`) to avoid breaking Docker command lines.
4. Deduplicate helpers:
   - Remove duplicate env parsing functions from `main.go` files.
5. Update docs/help output:
   - Ensure `--help` prints correct examples and env vars.
6. Validate behavior:
   - `go test ./...` (add tests only where patterns already exist).
   - Manual quick checks:
     - `go run ./cmd/devtool --help`
     - `go run ./cmd/devtool doctor`
     - `go run ./cmd/devtool once --url ...`

## Expected Wins

- Automatic help (`--help`) for every command, consistent usage and error handling.
- Easier to add new commands (e.g., “print Codex MCP config”) without editing a giant switch statement.
- Cleaner separation between command wiring and business logic.
- Better testability (commands can be executed with injected args and captured outputs).

## Non-goals (For This Refactor)

- Changing the crawler JSON schema/contract.
- Changing how Chrome/Codex/MCP are configured (that stays per `docs/shopee_crawler_plan.md`).
- Introducing Viper/Fx immediately (can be layered later if/when config grows).

## Risks / Constraints

- Adding Cobra requires updating `go.mod`, which requires **network access** to fetch modules.
- CLI compatibility: to avoid breaking Make/Docker, preserve existing flag names and command names during the first migration (Option A).

