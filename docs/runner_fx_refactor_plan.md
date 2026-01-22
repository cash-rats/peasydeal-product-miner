# Runner FX Refactor Plan

## Goal

Refactor `internal/runner` to be FX-friendly by:

- Constructing runners (`codex`, `gemini`) via `fx.Provide`
- Aggregating them into a `map[string]ToolRunner` (`name -> runner`) via `NewRunners`
- Selecting the runner in the crawl entrypoint based on `runner.Options.Tool`, with zero global singletons

## Current State (Summary)

- `internal/runner/runner.go` exports `RunOnce(opts Options)` which:
  - Detects source + selects prompt
  - Constructs a tool runner via `NewToolRunnerFromOptions(opts)`
  - Executes the tool and parses/validates JSON
  - Writes output file under `OutDir`
- `internal/runner/codex.go` and `internal/runner/gemini.go` implement `Name()` and `Run(url, prompt)`.
- Call sites exist in:
  - CLI commands (e.g. `cmd/devtool`)
  - Server-side domain code (e.g. `internal/app/inngest/crawl/crawl.go`)

## Design (Proposed)

### 1) Define the interface explicitly

Keep the runner contract in `internal/runner`:

- `type ToolRunner interface { Name() string; Run(url, prompt string) (string, error) }`

Both `*CodexRunner` and `*GeminiRunner` already satisfy this.

### 2) Aggregate runners with `NewRunners`

Prefer FX groups so adding a new tool does not require changing `NewRunners`’ signature.

- Each runner is provided into a group:
  - `fx.Provide(fx.Annotate(NewCodexRunner, fx.As(new(ToolRunner)), fx.ResultTags(\`group:"tool_runners"\`)))`
  - `fx.Provide(fx.Annotate(NewGeminiRunner, fx.As(new(ToolRunner)), fx.ResultTags(\`group:"tool_runners"\`)))`
- `NewRunners` collects the group and builds the map:
  - Input: `struct { fx.In; Runners []ToolRunner \`group:"tool_runners"\` }`
  - Output: `map[string]ToolRunner`
  - Behavior: key by `runner.Name()`; if duplicates, fail fast with an error.

### 3) Make `RunOnce` FX-consumable (without breaking CLI)

`fx.In` only works when FX is constructing/calling something, so `RunOnce(opts Options)` cannot directly “take in fx.In” unless it becomes:

- A method on an FX-provided type, or
- A function invoked by FX (e.g. `fx.Invoke(...)`)

Plan:

1. Introduce an FX-wired service:
   - `type Service struct { runners map[string]ToolRunner }`
   - `func NewService(runners map[string]ToolRunner) *Service`
   - `func (s *Service) RunOnce(opts Options) (string, Result, error)` that contains today’s logic but replaces `NewToolRunnerFromOptions(opts)` with:
     - `tr := s.runners[normalizedToolName(opts.Tool)]`
     - if missing: return an `errorResult` and write it, preserving existing behavior
2. Keep a backward-compatible top-level function for non-FX call sites:
   - `func RunOnce(opts Options) (string, Result, error)` becomes a thin wrapper that constructs a service using the existing behavior (see “Migration” below).

This keeps the CLI working while enabling FX injection for server code.

### 4) Tool selection rules

Define a single normalization function in `internal/runner`:

- Default: tool is `"codex"` when empty (preserve current default behavior in CLI flags)
- Accept case-insensitive values (`"Codex"`, `"CODEX"`, etc.)
- Return a helpful error when the tool is unknown, listing supported keys (from the runners map)

### 5) Runner configuration (Options vs FX config)

There is a tension to resolve:

- Today, `runner.Options` includes tool configuration (`Cmd`, `Model`, `SkipGitRepoCheck`)
- FX typically builds long-lived singletons (one runner instance per tool)

Two viable approaches:

**A) Keep runner instances configurable per-call (minimal FX coupling)**

- Runners are mostly stateless and accept command/model as part of `Options`, not constructor config.
- `Service.RunOnce` selects `ToolRunner` by name, then calls something like `RunWithOptions(url, prompt, opts)`.
- This requires changing the interface, so it is a larger signature change.

**B) Keep the existing runner interface; shift cmd/model to app config**

- Runners are constructed once with `cmd/model` from config (Viper/env defaulting).
- `runner.Options.Cmd/Model` become deprecated overrides (or removed later).
- `Service.RunOnce` only uses `opts.Tool` for selection.

Recommended starting point: **B**, to minimize interface churn (keep `Run(url, prompt)`).

Migration detail for B:

- Add a `runner.Config` (or reuse existing config structure) with defaults:
  - Codex: cmd default `"codex"`, model default `CODEX_MODEL` or `"gpt-5.2"`, skip flag default false
  - Gemini: cmd default `"gemini"`, model default empty, allowed MCP default `"chrome-devtools"`
- CLI call sites should set config via flags/env into the FX app (or continue using legacy `RunOnce` wrapper until CLI is migrated).

## Wiring (FX Module)

Add a module under `internal/runner/fx`:

- `internal/runner/fx/module.go`:
  - `var Module = fx.Options(...)` providing:
    - `NewCodexRunner(...)` as grouped `ToolRunner`
    - `NewGeminiRunner(...)` as grouped `ToolRunner`
    - `NewRunners(...)` producing `map[string]ToolRunner`
    - `NewService(...)` producing `*runner.Service`

Then, in server domains that need crawling (e.g. `internal/app/inngest/...`), depend on `*runner.Service` (or an interface) via FX.

## Call Site Migration

### Server-side (FX)

- Update `internal/app/inngest/crawl/...` to receive `*runner.Service` via `fx.In` and call `service.RunOnce(...)`.
- Ensure the domain module includes `internal/runner/fx.Module` (or the root `cmd/server/main.go` does).

### CLI-side (non-FX, initially)

Keep existing behavior by leaving `runner.RunOnce(opts)` callable.

- Phase 1 (fast): keep `runner.RunOnce` as-is, only refactor server-side to FX.
- Phase 2 (optional): move CLI commands to construct an `fx.App` that provides runner config and resolves `*runner.Service`.

## Validation Checklist

- `go test ./...` (or `go test ./...` plus any repo-specific make target)
- CLI: `cmd/devtool once` runs and writes output JSON
- Server: existing health endpoints still work; crawl code path can resolve runners via FX
- Error cases still write output files when possible (preserve current `RunOnce` semantics)
