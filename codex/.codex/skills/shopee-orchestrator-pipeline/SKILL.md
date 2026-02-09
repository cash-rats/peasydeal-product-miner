---
name: shopee-orchestrator-pipeline
description: Run the full Shopee snapshot-first pipeline (snapshot_capture->core_extract->images_extract->variations_extract->variation_image_map_extract), persist stage artifacts/state, and return one final contract JSON.
---

# Shopee Orchestrator Pipeline

You are a pipeline orchestrator for Shopee product crawling.

Your job is to run this exact sequence and persist artifacts after every stage:

`snapshot_capture (S0) -> core_extract (A) -> images_extract (B) -> variations_extract (C) -> variation_image_map_extract (D) -> final_merge`

## Critical Rules

1. JSON-only final output: return exactly one JSON object, no markdown/prose.
2. Single-navigation: only `snapshot_capture` may operate the real page via Chrome DevTools.
3. `core_extract`/`images_extract`/`variations_extract`/`variation_image_map_extract` are offline-only: read artifacts from disk, do not open/navigate pages.
4. Artifact discipline: every stage completion must write its stage artifact and update `_pipeline-state.json`.
5. Never write placeholder JSON (`...`, `{...}`, `[...]`). Always write valid JSON via structured serialization.
6. Browser cleanup is mandatory: when pipeline finishes (success/failure), ensure the crawl tab opened during `snapshot_capture` is closed.

## Required Artifact Directory

Create one run directory:

`out/artifacts/<run_id>/`

`run_id` should be deterministic and traceable, for example:
`YYYYMMDDThhmmssZ_<short_hash_or_rand>`

Required files:

- `out/artifacts/<run_id>/_pipeline-state.json`
- `out/artifacts/<run_id>/s0-snapshot-pointer.json`
- `out/artifacts/<run_id>/s0-page_state.json`
- `out/artifacts/<run_id>/s0-page.html`
- `out/artifacts/<run_id>/a-core.json`
- `out/artifacts/<run_id>/b-images.json`
- `out/artifacts/<run_id>/c-variations.json`
- `out/artifacts/<run_id>/d-variation-image-map.json`
- `out/artifacts/<run_id>/final.json`
- `out/artifacts/<run_id>/meta.json`

If a stage is skipped/disabled/failed, still write that stage file with an empty-but-valid object and an explanatory status.

## Pipeline State Contract

`_pipeline-state.json` must be updated at least:

- run initialized
- stage started
- stage completed/failed
- run finalized

Shape:

```json
{
  "run_id": "string",
  "url": "string",
  "started_at": "ISO-8601 UTC",
  "updated_at": "ISO-8601 UTC",
  "current_stage": "snapshot_capture|core_extract|images_extract|variations_extract|variation_image_map_extract|final_merge",
  "status": "running|completed|needs_manual|error",
  "stages": {
    "snapshot_capture": {"status": "pending|running|completed|skipped|error|needs_manual", "started_at": "", "ended_at": "", "error": ""},
    "core_extract": {"status": "pending|running|completed|skipped|error|needs_manual", "started_at": "", "ended_at": "", "error": ""},
    "images_extract": {"status": "pending|running|completed|skipped|error|needs_manual", "started_at": "", "ended_at": "", "error": ""},
    "variations_extract": {"status": "pending|running|completed|skipped|error|needs_manual", "started_at": "", "ended_at": "", "error": ""},
    "variation_image_map_extract": {"status": "pending|running|completed|skipped|error|needs_manual", "started_at": "", "ended_at": "", "error": ""}
  },
  "flags": {
    "images_enabled": true,
    "variations_enabled": true,
    "variation_image_map_enabled": true
  }
}
```

## Stage Skills to Follow

For stage methodology, read and follow these skill specs in this repo:

- `snapshot_capture`: `.agents/skills/shopee-page-snapshot/SKILL.md`
- `core_extract`: `.agents/skills/shopee-product-core/SKILL.md`
- `images_extract`: `.agents/skills/shopee-product-images/SKILL.md`
- `variations_extract`: `.agents/skills/shopee-product-variations/SKILL.md`
- `variation_image_map_extract`: `.agents/skills/shopee-variation-image-map/SKILL.md`

If downstream skill files are missing, continue with best-effort offline parsing from snapshot artifacts and record fallback usage in `meta.json`.

## Runtime Flags

Use runtime prompt values or defaults:

- `description_max_chars` default `1500`
- `images_max` default `20`
- `variations_max` default `20`
- `variation_image_map_max` default `10`
- `images_enabled` default `true`
- `variations_enabled` default `true`
- `variation_image_map_enabled` default `true`

Always enforce limits again at final merge.

## Stage Execution Requirements

### snapshot_capture (S0)

1. Start stage `snapshot_capture` in `_pipeline-state.json`.
2. Execute snapshot behavior using `shopee-page-snapshot` rules.
3. Persist snapshot outputs:
   - pointer -> `s0-snapshot-pointer.json`
   - page state -> `s0-page_state.json`
   - page html -> `s0-page.html`
4. If snapshot status is `needs_manual`: finalize pipeline with final status `needs_manual`.
5. If snapshot status is `error`: finalize pipeline with final status `error`.
6. Enforce tab finalization from snapshot stage:
   - The tab opened for this crawl must be closed before leaving `snapshot_capture`.
   - If still open due to an interruption, close it during pipeline finalization.

### core_extract (A, offline)

1. Start stage `core_extract`.
2. Parse snapshot artifacts for `title`, `description`, `currency`, `price`, `status`.
3. Write `a-core.json`.
4. Gate rule:
   - `core_extract=needs_manual` -> stop pipeline, final=`needs_manual`
   - `core_extract=error` -> stop pipeline, final=`error`
   - `core_extract=ok` -> continue

### images_extract (B, offline)

1. Start stage `images_extract`.
2. Extract image URLs from snapshot artifacts.
3. Deduplicate and cap by `images_max`.
4. Write `b-images.json`.
5. On failure: do not fail whole pipeline; write error note and continue.

### variations_extract (C, offline)

1. Start stage `variations_extract`.
2. Extract variation options (`title`, `position`) from snapshot artifacts.
3. Cap by `variations_max`.
4. Write `c-variations.json`.
5. On failure: do not fail whole pipeline; write error note and continue.

### variation_image_map_extract (D, offline)

1. Start stage `variation_image_map_extract`.
2. Best-effort map variation -> image from snapshot artifacts.
3. Process at most `variation_image_map_max` options.
4. Single option failure must be skipped, not hard-fail stage.
5. Write `d-variation-image-map.json`.

## Merge Rules

Build final output from `core_extract` as base, then merge:

1. Base from `core_extract` (`status/url/captured_at/title/description/currency/price`).
2. Merge `images_extract` into `images`.
3. Merge `variations_extract` into `variations`.
4. Merge `variation_image_map_extract` by matching `title` + `position`.
5. If non-core stages fail, keep degraded output (`images=[]`, `variations=[]` or partial) and keep final `status="ok"` when `core_extract=ok`.

## Final Output Contract

Return exactly one JSON object:

```json
{
  "url": "string",
  "status": "ok|needs_manual|error",
  "captured_at": "ISO-8601 UTC",
  "notes": "string",
  "error": "string",
  "title": "string",
  "description": "string",
  "currency": "string",
  "price": "number|string",
  "images": ["string"],
  "variations": [{"title":"string","position":0,"image":"string"}],
  "artifact_dir": "out/artifacts/<run_id>",
  "run_id": "string"
}
```

Rules:

- Always include `images` and `variations` (use `[]` when empty).
- `status=ok` requires core fields (`title/description/currency/price`).
- `status=needs_manual` requires non-empty `notes`.
- `status=error` requires non-empty `error`.
- Save this same object to `final.json` before printing.

## meta.json

Write run diagnostics:

```json
{
  "run_id": "string",
  "orchestrator_skill": "shopee-orchestrator-pipeline",
  "stage_duration_ms": {
    "snapshot_capture": 0,
    "core_extract": 0,
    "images_extract": 0,
    "variations_extract": 0,
    "variation_image_map_extract": 0
  },
  "stage_errors": [],
  "limits": {
    "description_max_chars": 1500,
    "images_max": 20,
    "variations_max": 20,
    "variation_image_map_max": 10
  },
  "fallbacks": []
}
```

## Recovery Rules

- If any JSON artifact decode fails, do not crash silently:
  - record stage error in `_pipeline-state.json` and `meta.json`
  - continue when allowed by degradation policy
- Never skip writing state/artifact files due to partial failure.
- Before returning final JSON, run a final browser-tab cleanup check and close any crawl tab left by this run.
- Do not emit extra text in stdout beyond final JSON.
