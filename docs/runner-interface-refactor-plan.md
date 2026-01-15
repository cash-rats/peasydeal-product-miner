# Runner Interface Refactor Plan (Codex/Gemini)

## Goal
Refactor `internal/runner.RunOnce` so tool-specific execution logic lives behind a small interface implemented by `CodexRunner` and `GeminiRunner`, while `RunOnce` remains a tool-agnostic orchestration function (detect source → load prompt → run tool → parse/validate → write output).

## Motivation (current pain points)
- `internal/runner/runner.go` mixes orchestration with tool details (CLI flags, log lines, error strings).
- Options naming is tool-biased (`CodexCmd` is also used for Gemini), which makes the API confusing.
- Error messaging is tool-inaccurate (`parseResult` says “from codex” even for Gemini).
- Harder to add a 3rd tool or unit-test argument construction without touching `RunOnce`.

## Non-goals
- No behavior changes to crawling/prompt selection/result schema unless explicitly called out.
- No FX wiring changes (this is a local refactor inside `internal/runner` + callers).
- No network/API integration changes; runners remain “shell out to CLI” wrappers.

## Proposed design

### 1) Introduce a `Runner` interface
Keep it intentionally small and focused on “execute tool and return raw JSON string”:

- Place the interface in `internal/runner/` (same package as `RunOnce`), ideally in a dedicated file like `internal/runner/tool_runner.go`.
- Prefer naming it by behavior (Go convention) rather than `RunnerInterface`. Suggested name: `ToolRunner` (or `Runner` if you don’t anticipate other runner concepts).

- `type ToolRunner interface {`
  - `Name() string`
  - `Run(prompt string) (raw string, err error)`
  - `}`

Optional (nice-to-have) if we want cancellation/timeouts soon:
- `Run(ctx context.Context, prompt string) (raw string, err error)`

### 2) Add concrete implementations
Add two structs in `internal/runner/` (likely using the existing placeholder files `codex.go` and `gemini.go`):

- `type CodexRunner struct {`
  - `Cmd string` (binary path/name)
  - `Model string`
  - `SkipGitRepoCheck bool`
  - `}`
- `type GeminiRunner struct {`
  - `Cmd string`
  - `Model string`
  - `}`

Responsibilities of each implementation:
- Build CLI args and call `exec.Command`.
- Capture stdout/stderr and return a clean error message on failure.
- Do not parse JSON; only return raw stdout string (trimmed).

### 3) Add a factory that maps `Options` → `ToolRunner`
Add a helper in `internal/runner`:
- `func NewToolRunnerFromOptions(opts Options) (ToolRunner, error)`

It handles:
- Defaulting `opts.Tool` (default `"codex"`).
- Defaulting the command/binary path.
- Validating tool selection (`"codex"` or `"gemini"`).

### 4) Make `RunOnce` tool-agnostic
Refactor `RunOnce` so it:
- Validates `URL`/`OutDir`, selects prompt file, loads prompt (as today).
- Constructs `runner := NewToolRunnerFromOptions(opts)`.
- Calls `raw, err := runner.Run(prompt)`.
- Parses and validates the result and writes the output file (as today).

Tool-specific log lines move into each runner (or `RunOnce` logs `runner.Name()` and avoids printing the full prompt).

## Options/API changes (keep callers stable)

### Minimal-churn approach (recommended)
- Keep `Options.Tool` as-is (`"codex"` / `"gemini"`).
- Introduce a tool-neutral command field and keep compatibility:
  - Add `Options.Cmd string` (new preferred name).
  - Keep `Options.CodexCmd string` but treat it as deprecated alias:
    - If `Cmd` is empty, use `CodexCmd`.
    - Update call sites gradually to set `Cmd` instead of `CodexCmd`.

This avoids breaking:
- `cmd/runner/cmd/root.go` (currently only sets `CodexCmd`)
- `cmd/devtool/cmd/once.go`
- `internal/app/inngest/crawl/crawl.go`

### Follow-up cleanup (optional)
After callers migrate to `Options.Cmd`, remove `CodexCmd` in a later PR.

## Logging improvements (optional but high-value)
Current log prints the full prompt content. Suggested change:
- Log `prompt_file` and `tool`/`model` instead of the prompt body.
- If debugging is needed, log prompt length or a short hash (not the full text).

## Parsing/error message fixes (small correctness tweaks)
- Update `parseResult` error to be tool-agnostic (e.g. “invalid JSON from tool”) or include `runner.Name()`.
- Keep `validateMinimal` behavior unchanged.

## Testing strategy
If there are existing tests for `internal/runner`, add unit tests for:
- Argument construction per runner (codex vs gemini).
- Error formatting when stderr has content.

To make tests feasible without spawning real CLIs, inject command execution:
- Add `execCommand func(name string, args ...string) *exec.Cmd` to each runner (defaulting to `exec.Command`), so tests can stub it.

If the repo currently has no Go tests, keep this refactor testless but structured to enable tests later.

## Step-by-step execution plan
1. Define `ToolRunner` interface in `internal/runner/tool_runner.go`.
2. Implement `CodexRunner` and `GeminiRunner` in `internal/runner/codex_runner.go` and `internal/runner/gemini_runner.go` (or reuse `codex.go` / `gemini.go` by renaming them).
3. Add `NewToolRunnerFromOptions(opts Options)` factory and migrate defaults (`Tool`, command path, model flags).
4. Refactor `RunOnce` to use the interface; delete/inline `runCodex` and `runGemini` helpers if they become redundant.
5. Make parsing error messages tool-neutral (or tool-specific via `runner.Name()`).
6. Update callers to use `Options.Cmd` (keeping `CodexCmd` as backward-compatible alias during transition).
7. Run `go test ./...` (and/or the repo’s usual validation commands) to confirm no behavior regressions.

## Acceptance criteria
- `RunOnce` contains no tool-specific CLI argument building.
- `CodexRunner` and `GeminiRunner` each fully encapsulate their command invocation details.
- Existing CLIs and the Inngest flow still call `RunOnce` successfully without changing required env vars.
- Output JSON schema and output file writing behavior remain unchanged.
