# Runner JSON Enforcement Plan (Gemini + Codex)

## Context / Current State

- `internal/runner/gemini.go`:
  - Unwraps Gemini CLI `-o json` wrapper via `unwrapGeminiJSON()`.
  - Extracts first JSON object via `extractFirstJSONObject()` (decoder-based).
  - On failure, does one repair pass (second Gemini call) and retries extraction.
  - Returns canonical JSON object string (the extracted object).
- `internal/runner/codex.go`:
  - Executes Codex CLI and returns raw stdout (no extraction/repair).
- `internal/runner/runner.go`:
  - Parses tool output using `parseResult()` which calls `extractFirstJSONObject()` (applies to all tools).
  - Validates prompt-defined contract via `validateContract()` (CrawlOut + validator/v10).

## Goal

Guarantee that **both** Gemini and Codex tool runs produce a **single valid JSON object** matching the prompt-defined output contract, with at most **one** repair attempt per run.

## Non-Goals

- No separate JSON schema file maintained as source-of-truth.
- No additional tool runners added in this plan.

## Approach (Recommended): Mirror GeminiRunner in CodexRunner

**Idea:** Each runner returns canonical JSON or errors; `runner.go` stays simple.

1) In `internal/runner/codex.go`:
   - Add `runModelText(url, prompt)` helper (same pattern as `GeminiRunner.runModelText`).
   - After getting stdout (model text), call `extractFirstJSONObject(modelText)`:
     - On success: return the extracted canonical JSON object string.
     - On failure: do exactly **one** repair call to Codex using a repair prompt template (same pattern as Gemini).
   - Keep all Codex-specific concerns in this file (flags, model selection, skip-git-repo-check, etc.).

2) In `internal/runner/gemini.go`:
   - Keep the current behavior as the reference implementation:
     - wrapper unwrap (`unwrapGeminiJSON`)
     - `extractFirstJSONObject`
     - one repair call
     - return canonical JSON object string

3) In `internal/runner/runner.go`:
   - Keep `parseResult()` + `validateContract()` as a final “belt-and-suspenders” check.
   - Remove any tool-name branching (Gemini/Codex repair should not live here).

## Proposed Implementation Steps

### Step 1: Add Codex extraction + one-time repair (copy Gemini pattern)

- Implement in `internal/runner/codex.go`:
  - `runModelText(...)` (exec wrapper)
  - `buildCodexRepairPrompt(...)` (same contract text as Gemini repair prompt)
  - `Run(...)` flow:
    - `modelText := runModelText(...)`
    - try `extractFirstJSONObject(modelText)`
    - if fail: run once with repair prompt and retry `extractFirstJSONObject`
    - return extracted JSON string (canonical single object)

#### Codex CLI invocation (must match current behavior)

Reuse the current Codex runner calling convention:
- Command: `codex exec ... "<prompt>"`
- Flags:
  - include `--skip-git-repo-check` when configured
  - include `--model <model>` when non-empty (or fall back to `CODEX_MODEL` / default)

#### Codex `Run` pseudocode (concrete)

```go
func (r *CodexRunner) Run(url, prompt string) (string, error) {
  modelText, err := r.runModelText(url, prompt)
  if err != nil { return "", err }

  extracted, err := extractFirstJSONObject(modelText)
  if err == nil {
    r.logCodexOutput(url, modelText) // debug + truncation
    return extracted, nil
  }

  r.logger.Infow("runner_codex_repair_attempt", "url", url, "err", err.Error())
  repairPrompt := buildCodexRepairPrompt(url, modelText)

  repairedText, rerr := r.runModelText(url, repairPrompt)
  if rerr != nil {
    r.logger.Infow("runner_codex_repair_failed", "url", url, "err", rerr.Error())
    return "", fmt.Errorf("codex returned non-JSON output: %w", err)
  }

  repairedExtracted, perr := extractFirstJSONObject(repairedText)
  if perr != nil {
    r.logger.Infow("runner_codex_repair_failed", "url", url, "err", perr.Error())
    return "", fmt.Errorf("codex returned non-JSON output: %w", err)
  }

  r.logger.Infow("runner_codex_repair_succeeded", "url", url)
  r.logCodexOutput(url, repairedText)
  return repairedExtracted, nil
}
```

Notes:
- The runner should return the **extracted JSON object string**, not raw stdout.
- Keep repair to **one** attempt (no loops/recursion).
- Use `extractFirstJSONObject()` from `internal/runner/json_extract.go`.

### Step 2: Keep `runner.go` generic

- Ensure `internal/runner/runner.go` does not contain tool-specific repair logic.
- Leave `parseResult()` + `validateContract()` in place for final validation.
- After Codex/Gemini return canonical JSON strings, `parseResult()` becomes a final guard and should usually succeed.

### Step 3: Tests

- Add unit tests in `internal/runner/codex_test.go` (or extend existing tests) with an injected `execCommand` stub:
  - Case 1: first call returns valid JSON -> no repair call
  - Case 2: first call returns non-JSON, second returns JSON -> repair succeeds
  - Case 3: both calls non-JSON -> returns error
- Keep existing extraction tests (`internal/runner/json_extract_test.go`) and contract tests (`internal/runner/contract_test.go`).

## Logging requirements (Codex)

Match the Gemini logging style and use zap only:
- `crawl_started` / `crawl_finished` / `crawl_failed` with fields: `tool=codex`, `url`, `duration` (string)
- `runner_codex_repair_attempt` / `runner_codex_repair_succeeded` / `runner_codex_repair_failed`
- `llm_output` at debug level with truncation (e.g. 8000 chars)

## Acceptance Criteria

- Codex and Gemini runs:
  - Return exactly one JSON object (canonicalized) on success.
  - Attempt at most one repair call when output is non-JSON or violates contract.
  - On repeated failure, return an error that surfaces the parse/validation failure reason.
- Logs:
  - Repair attempt/success/failure logs include `tool` and `url`.

## Rollout Notes

- Start by enabling repair for Codex only (Gemini already has repair).
- Keep prompts strict (JSON-only, single object) to reduce repair frequency.
