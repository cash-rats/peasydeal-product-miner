---
name: taobao-orchestrator-pipeline
description: Run the full Taobao snapshot-first pipeline (snapshot_capture->core_extract->images_extract->variations_extract->variation_image_map_extract), persist stage artifacts/state, and return one final contract JSON.
---

# Taobao Orchestrator Pipeline

You are a pipeline orchestrator for Taobao product crawling.

Run this exact sequence:

`snapshot_capture (S0) -> core_extract (A) -> images_extract (B) -> variations_extract (C) -> variation_image_map_extract (D) -> final_merge`

## Critical Rules

1. Final stdout must be exactly one JSON object (no markdown/prose).
2. Only `snapshot_capture` may use browser interaction.
3. `core_extract` / `images_extract` / `variations_extract` / `variation_image_map_extract` are offline-only and must read artifacts from disk.
4. Every stage must persist its stage artifact and update `_pipeline-state.json`.
5. `final.json` and stdout JSON must be exactly the same object.
6. Taobao output contract must match Shopee conventions.
7. If A(core) is `ok`, B/C/D failures must be degraded and still return final `status="ok"`.

## Runtime Helper (required)

Use the local helper script for orchestration:

```bash
python3 ./scripts/run_pipeline.py --artifact-dir out/artifacts/<run_id>
```

Optional limits/flags:

```bash
python3 ./scripts/run_pipeline.py \
  --artifact-dir out/artifacts/<run_id> \
  --images-max 20 \
  --variations-max 20 \
  --variation-image-map-max 20 \
  --description-max-chars 1500
```

The helper script must:

- execute stage order exactly `S0 -> A -> B -> C -> D -> merge`
- write `_pipeline-state.json`, `meta.json`, `final.json`
- print only final JSON to stdout

## Required Artifact Directory

`out/artifacts/<run_id>/`

Required files after pipeline:

- `_pipeline-state.json`
- `s0-snapshot-pointer.json`
- `s0-manifest.json`
- `s0-initial.html.gz`
- `s0-overlay.html.gz` (best-effort)
- `s0-variation-<position>.html.gz` (best-effort)
- `core_extract.json`
- `images_extract.json`
- `variations_extract.json`
- `variation_image_map_extract.json`
- `meta.json`
- `final.json`

## Stage Mapping

- `snapshot_capture`: `taobao-page-snapshot`
- `core_extract`: `taobao-product-core`
- `images_extract`: `taobao-product-images`
- `variations_extract`: `taobao-product-variations`
- `variation_image_map_extract`: `taobao-variation-image-map`

## Gate / Degrade Rules

- S0 `needs_manual` => finalize `needs_manual`
- S0 `error` => finalize `error`
- A `needs_manual` => finalize `needs_manual`
- A `error` => finalize `error`
- B/C/D `error` => keep degraded arrays, continue, final remains `ok` when A is `ok`

## Final Output Contract (JSON only)

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
  "variations": [{"title":"string","position":0,"price":"number|string","images":["string"]}],
  "artifact_dir": "out/artifacts/<run_id>",
  "run_id": "string"
}
```

Rules:

- always include `images` and `variations`
- every variation item must include `price` and `images`
- `status=needs_manual` requires non-empty `notes`
- `status=error` requires non-empty `error`
- save the same object to `final.json` before printing
