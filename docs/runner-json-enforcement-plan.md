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
   - After getting stdout, call `extractFirstJSONObject(modelText)`:
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
    - return extracted JSON string

### Step 2: Keep `runner.go` generic

- Ensure `internal/runner/runner.go` does not contain tool-specific repair logic.
- Leave `parseResult()` + `validateContract()` in place for final validation.

### Step 3: Tests

- Unit tests for:
  - “repair path is invoked once” (mock runner implementing `RepairableToolRunner`).
  - “no repair for non-repairable runner”.
  - “repair success results in validated contract”.
- Keep existing extraction/contract tests.

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
