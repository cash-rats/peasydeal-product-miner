---
name: shopee-orchestrator-pipeline
description: Run the full Shopee snapshot-first pipeline (snapshot_capture->core_extract->images_extract->variations_extract->variation_image_map_extract), persist stage artifacts/state, and return one final contract JSON.
---

# Shopee Orchestrator Pipeline

You are a pipeline orchestrator for Shopee product crawling.

Run this exact sequence:

`snapshot_capture (S0) -> core_extract (A) -> images_extract (B) -> variations_extract (C) -> variation_image_map_extract (D) -> final_merge`

## Critical Rules

1. Final stdout must be exactly one JSON object (no markdown/prose).
2. Only `snapshot_capture` may use browser interaction.
3. `core_extract` / `images_extract` / `variations_extract` / `variation_image_map_extract` are offline-only and must read artifacts from disk.
4. Every stage must persist its stage artifact and update `_pipeline-state.json`.
5. Always write valid JSON objects. Never use placeholder JSON.
6. Ensure crawl tab cleanup before pipeline exit.
7. A/B/C/D must parse HTML-based artifacts from S0 (not legacy `s0-page_state.json`).

## Required Artifact Directory

Create one run directory:

`out/artifacts/<run_id>/`

Required files:

- `_pipeline-state.json`
- `s0-snapshot-pointer.json`
- `s0-manifest.json`
- `s0-initial.html.gz`
- `s0-overlay.html.gz` (best-effort)
- `s0-variation-<position>.html.gz` (best-effort, first 20)
- `core_extract.json`
- `images_extract.json`
- `variations_extract.json`
- `variation_image_map_extract.json`
- `final.json`
- `meta.json`

If a stage fails/skips, still write a valid stage artifact with status/error fields.

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

Resolve stage skill file by first existing path:

1. `$HOME/.codex/skills/<skill_name>/SKILL.md`
2. `/codex/.codex/skills/<skill_name>/SKILL.md`
3. `codex/.codex/skills/<skill_name>/SKILL.md`

Stage mapping:

- `snapshot_capture`: `shopee-page-snapshot`
- `core_extract`: `shopee-product-core`
- `images_extract`: `shopee-product-images`
- `variations_extract`: `shopee-product-variations`
- `variation_image_map_extract`: `shopee-variation-image-map`

If any required stage skill file is missing, fail fast:

1. set pipeline status=`error`
2. write `_pipeline-state.json` with missing-skill details
3. write `final.json` with non-empty `error`
4. return final JSON immediately

## Runtime Limits

Defaults:

- `description_max_chars = 1500`
- `images_max = 20`
- `variations_max = 20`
- `variation_image_map_max = 20`
- `images_enabled = true`
- `variations_enabled = true`
- `variation_image_map_enabled = true`

Enforce limits again at final merge.

## Stage Execution Requirements

### snapshot_capture (S0)

1. Start stage `snapshot_capture`.
2. Execute `shopee-page-snapshot`.
3. Persist snapshot outputs (`s0-snapshot-pointer.json`, `s0-manifest.json`, html snapshots).
4. If `status=needs_manual`: finalize pipeline with `needs_manual`.
5. If `status=error`: finalize pipeline with `error`.
6. Ensure crawl tab is closed before leaving stage.

### core_extract (A, offline)

1. Start stage `core_extract`.
2. Run `shopee-product-core` against `artifact_dir`.
3. Write `core_extract.json`.
4. Gate rule:
   - `needs_manual` => stop pipeline with `needs_manual`
   - `error` => stop pipeline with `error`
   - `ok` => continue

### images_extract (B, offline)

1. Start stage `images_extract`.
2. Run `shopee-product-images` against `artifact_dir`.
3. Write `images_extract.json`.
4. On failure: keep degraded output and continue.

### variations_extract (C, offline)

1. Start stage `variations_extract`.
2. Run `shopee-product-variations` against `artifact_dir`.
3. Write `variations_extract.json`.
4. On failure: keep degraded output and continue.

### variation_image_map_extract (D, offline)

1. Start stage `variation_image_map_extract`.
2. Run `shopee-variation-image-map` against `artifact_dir`.
3. Write `variation_image_map_extract.json`.
4. On failure: keep degraded output and continue.

## Merge Rules

Build final output from `core_extract.json` as base:

1. Base fields from `core_extract.json`: `status,title,description,currency,price`.
2. Merge `images_extract.json.images` into `images` (dedupe, cap `images_max`).
3. Merge `variations_extract.json.variations` into `variations` (cap `variations_max`).
4. Merge `variation_image_map_extract.json.variations` by (`title`,`position`) and attach `images` per variation.
5. If B/C/D fail but A is `ok`, keep final `status="ok"` with degraded arrays.

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
  "variations": [{"title":"string","position":0,"images":["string"]}],
  "artifact_dir": "out/artifacts/<run_id>",
  "run_id": "string"
}
```

Rules:

- Always include `images` and `variations`.
- Every variation item must include `images` (use `[]` when empty).
- `status=ok` requires core fields.
- `status=needs_manual` requires non-empty `notes`.
- `status=error` requires non-empty `error`.
- Save the same object to `final.json` before printing.

## meta.json

Write diagnostics:

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
    "variation_image_map_max": 20
  },
  "fallbacks": []
}
```

## Recovery Rules

- If JSON decode fails, record stage error in `_pipeline-state.json` + `meta.json`, then continue only when degradation policy allows.
- Missing stage skill file is fatal (`status=error`).
- Never skip writing state/artifact files after partial failures.
- Before returning final JSON, perform final crawl-tab cleanup.
- Do not print any extra text beyond final JSON.
