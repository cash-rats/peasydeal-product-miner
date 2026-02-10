# PRD: Shopee Snapshot Pipeline v2 (S0 HTML-Only Artifacts)

## 1. Background and Problem

Current S0 (`shopee-page-snapshot`) handles:

1. Browser interaction (open page, open image overlay, switch variations)
2. Structured extraction (title/price/images/variations)
3. JSON artifact generation

This causes:

- Frequent large-output truncation risk when LLM emits long payloads
- Longer S0 runtime and larger failure surface
- Tight coupling between S0 and downstream extraction logic

## 2. Goal

Change S0 into an interaction + DOM snapshot stage only. S0 should not perform product-level structured extraction.

Downstream skills (`core/images/variations`) must parse HTML artifacts offline.

## 3. Non-Goals

- No selector quality optimization overhaul in this phase
- No cross-site generic extraction framework in this phase
- No DB schema or external API contract changes in this phase

## 4. User Stories

1. As a pipeline owner, I want S0 to complete quickly and reliably by writing raw HTML artifacts instead of large structured payloads.
2. As a downstream skill developer, I want to re-run extraction offline from artifacts without reopening a browser session.
3. As an operator, I want per-state HTML snapshots for debugging extraction failures.

## 5. Solution Overview

### 5.1 S0 Responsibilities (Snapshot-Only)

- Execute interaction flow (page load, open image overlay, switch first 10 variations)
- Call Python helper to fetch raw HTML from the current browser state and write artifacts
- Return only a small JSON pointer to artifacts

### 5.2 Artifact Contract (v2)

`out/artifacts/<run_id>/`:

- `s0-initial.html.gz`: initial product page state
- `s0-overlay.html.gz`: state after opening image overlay
- `s0-variation-<position>.html.gz`: state after selecting each of first 10 variation options (best-effort)
- `s0-manifest.json`: metadata only (no structured product fields)
  - `url`, `captured_at`, `run_id`, `blocked`, `notes`
  - `files[]` entries: `name`, `bytes`, `sha256`, `state_tag`
- `s0-snapshot-pointer.json`: small output pointer for orchestration

Notes:

- HTML files are gzip-compressed by default.
- S0 must not emit raw HTML to stdout.

### 5.3 Downstream Skill Updates

- `shopee-product-core`: parse `s0-initial.html(.gz)` for title/description/currency/price
- `shopee-product-images`: parse `s0-overlay.html(.gz)` first; fallback to initial HTML
- `shopee-product-variations`: parse initial and variation snapshots for options
- (If enabled) `shopee-variation-image-map`: derive mapping from `s0-variation-*.html.gz`

## 6. Functional Requirements

1. S0 must complete in a single browser session.
2. S0 must write HTML artifacts to disk; no large structured extraction output.
3. S0 stdout must remain a single small JSON object.
4. Partial success semantics must be supported (best-effort for overlay/variation states).
5. For login/verification/CAPTCHA walls, S0 returns `status="needs_manual"` and still writes available artifacts.

## 7. Non-Functional Requirements

- Performance: S0 mean runtime should decrease versus current baseline (target: >= 30% faster)
- Reliability: truncate-related failures should approach zero
- Reproducibility: all downstream parsing can run offline from artifacts only
- Observability: manifest must provide enough file metadata for validation and debugging

## 8. Acceptance Criteria

1. S0 no longer writes:
   - `s0-page_state.json`
   - `s0-overlay_images.json`
   - `s0-variations.json`
   - `s0-variation_image_map.json`
2. S0 always writes:
   - `s0-manifest.json`
   - `s0-snapshot-pointer.json`
3. S0 writes at least `s0-initial.html.gz`; overlay/variation artifacts are best-effort.
4. Downstream `core/images/variations` skills can run offline without opening browser pages.
5. Both blocked and non-blocked flows return clear `status/notes/error` semantics.

## 9. Risks and Mitigations

1. HTML structure drift on Shopee
- Mitigation: build shared parser helpers to centralize selector maintenance.

2. Extra cost from variation snapshots
- Mitigation: hard cap at first 10 options; skip on timeout and record notes.

3. Compression/read failures
- Mitigation: one shared utility for gzip read/write + hash + validation.

## 10. Milestones

1. M1: Finalize artifact v2 contract and filenames
2. M2: Convert S0 to snapshot-only and integrate Python HTML capture helper
3. M3: Update downstream core/images/variations skills to HTML-offline parsing
4. M4: End-to-end validation, regression checks, and docs updates

## 11. Success Metrics

- Mean S0 runtime reduction: >= 30%
- Truncation/oversized-output failure reduction: >= 90%
- Downstream offline extraction success rate: >= 95%
- Manifest-to-file integrity consistency: >= 99%

## 12. Migration Checklist

1. Update `shopee-page-snapshot` skill contract to HTML-only artifacts
2. Add Python capture helper and document invocation contract
3. Update downstream skills to allow HTML parsing (remove current "Do not parse HTML artifacts" restriction)
4. Keep stdout output JSON shape stable for orchestrator compatibility
5. Add test fixtures for:
   - normal product page
   - blocked/login wall page
   - missing overlay or missing variation edge cases
6. Update pipeline docs and runbook
