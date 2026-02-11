# PRD: Shopee Orchestrator Artifact-First (No Retry)

## 1. Summary

For Shopee skill-mode crawling (`shopee-orchestrator-pipeline`), the program must stop relying on LLM stdout JSON and use `out/artifacts/<run_id>/final.json` as the single source of truth.

Retry mechanisms must be removed.

---

## 2. Problem

Current behavior still depends on model stdout parsing. If stdout is truncated, the runner may trigger retry/repair paths. This introduces non-deterministic behavior and makes success dependent on output-length constraints.

The orchestrator skill already defines `final.json` as the final contract artifact, so runtime should trust that artifact directly.

---

## 3. Goals

1. Use `final.json` as the canonical crawl result in orchestrator skill mode.
2. Remove retry flows (truncation retry and repair retry).
3. Make failures explicit and deterministic.

---

## 4. Scope

In scope:

- `prompt_mode=skill`
- `skill_name=shopee-orchestrator-pipeline` (including default skill resolution for Shopee)
- Result loading from `out/artifacts/<run_id>/final.json`
- Erroring when `final.json` is missing/invalid/error-status

Out of scope:

- Legacy prompt mode behavior
- Non-Shopee skill-mode behavior

---

## 5. Functional Requirements

### FR-1: Run ID requirement

- In orchestrator skill mode, `run_id` is required.
- If missing, return error result immediately.

### FR-2: Artifact-first result loading

- After tool execution, read:
  - `out/artifacts/<run_id>/final.json`
- Parse this file as the crawler result payload.

### FR-3: Error handling

Return error when:

1. `final.json` cannot be found/read.
2. `final.json` is not valid JSON.
3. `final.json.status == "error"`.

### FR-4: Success handling

- If `final.json` is readable/valid and `status == "ok"`, treat run as success.

### FR-5: Retry removal

- Remove all Gemini retry behaviors for invalid/truncated JSON:
  - truncation retry
  - repair retry

No secondary LLM call should be triggered for JSON repair.

---

## 6. Status Semantics

Required behavior for this PRD:

- `status == "ok"`: success
- `status == "error"`: error

Open decision:

- `status == "needs_manual"`:
  - Option A: treat as error in worker pipeline.
  - Option B: pass through as non-ok terminal state.

If not decided separately, default recommendation is **Option B** to preserve current contract meaning.

---

## 7. Logging and Observability

When artifact-first path is used, include:

- `result_source=artifact_final`
- `artifact_final_path=<resolved path>`
- explicit error reason for artifact read/parse/status failures

Remove logs tied to retry decisions in no-retry mode.

---

## 8. Acceptance Criteria

1. Given valid `final.json` with `status=ok`, run returns success without parsing LLM stdout result payload.
2. Given missing `run_id`, run returns error.
3. Given missing `final.json`, run returns error.
4. Given invalid `final.json`, run returns error.
5. Given `final.json.status=error`, run returns error.
6. No retry/repair model invocation occurs after first model execution in this mode.

---

## 9. Risks

1. If skill execution does not write `final.json`, failures surface immediately (intended).
2. Requires strong run-id consistency between runner prompt context and artifact writer.
3. Any latent dependency on stdout parsing in this mode will be exposed during rollout.

---

## 10. Implementation Notes (High-level)

1. Make orchestrator skill mode use artifact-first result loading.
2. Keep contract validation on parsed `final.json`.
3. Remove Gemini retry/repair branches, keeping single-pass execution behavior.
4. Add/update tests for:
   - artifact success
   - missing run_id
   - missing/invalid final.json
   - final status error
   - no retry invocation path

