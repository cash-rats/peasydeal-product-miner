# Taobao Skills Handoff (For Next Agent)

## Scope and Current Status
- Goal: build Taobao skills incrementally under `codex/.codex`, following PRD checklist.
- Output contract rule: Taobao skills must match Shopee skills output conventions.
- Current progress:
  - `12.1 Preflight`: completed.
  - `12.2 taobao-page-snapshot`: completed and validated.
  - `12.3 taobao-product-core`: completed and validated.
  - `12.4 taobao-product-images`: completed and validated.
  - `12.5 taobao-product-variations`: completed and validated.
  - `12.6 taobao-variation-image-map`: completed and validated.
  - `12.7 taobao-orchestrator-pipeline`: completed and validated.

## User Decisions (Must Keep)
- New Taobao skills are created under `codex/.codex`.
- If crawler interaction is unclear or key data cannot be extracted, stop and ask user.
- Phase-1 variation strategy: only interact with `颜色分类` (not full multi-dimension combinations yet).

## Key Files
- PRD: `docs/prd_taobao_skill_incremental_validation.md`
- Test URLs: `codex/.codex/taobao_test_urls.md`
- S0 skill:
  - `codex/.codex/skills/taobao-page-snapshot/SKILL.md`
  - `codex/.codex/skills/taobao-page-snapshot/scripts/cdp_snapshot_html.py`
  - `codex/.codex/skills/taobao-page-snapshot/scripts/build_s0_outputs.py`
- Core skill:
  - `codex/.codex/skills/taobao-product-core/SKILL.md`
  - `codex/.codex/skills/taobao-product-core/scripts/extract_core_from_html.py`
- Images skill:
  - `codex/.codex/skills/taobao-product-images/SKILL.md`
  - `codex/.codex/skills/taobao-product-images/scripts/extract_images_from_html.py`
- Variations skill:
  - `codex/.codex/skills/taobao-product-variations/SKILL.md`
  - `codex/.codex/skills/taobao-product-variations/scripts/extract_variations_from_html.py`
- Variation image map skill:
  - `codex/.codex/skills/taobao-variation-image-map/SKILL.md`
  - `codex/.codex/skills/taobao-variation-image-map/scripts/extract_variation_image_map_from_html.py`
- Orchestrator skill:
  - `codex/.codex/skills/taobao-orchestrator-pipeline/SKILL.md`
  - `codex/.codex/skills/taobao-orchestrator-pipeline/scripts/run_pipeline.py`

## Commits Landed
- `8678f54`: `feat(taobao-s0): add taobao page snapshot skill`
- `b72f9dc`: `feat(taobao-core): add product-core skill and complete PRD 12.3`
- `ad6b19c`: `feat(taobao-images): add product-images skill and complete PRD 12.4`
- `8f0b9d4`: `feat(taobao-variations): add product-variations skill and complete PRD 12.5`
- `9231e29`: `feat(taobao-variation-image-map): add mapping skill and complete PRD 12.6`

## Validation Evidence
### Shared artifact runs
- OK run artifact dir: `out/artifacts/taobao-s0-ok-20260220-1332`
- Needs-manual run artifact dir: `out/artifacts/taobao-s0-needsmanual-20260220-1332`

### 12.2 S0 snapshot
- OK run has:
  - `s0-initial.html.gz`
  - `s0-overlay.html.gz`
  - `s0-variation-0.html.gz`
  - `s0-variation-1.html.gz`
  - `s0-manifest.json`
  - `s0-snapshot-pointer.json`
- Needs-manual run has:
  - `s0-initial.html.gz`
  - `s0-manifest.json`
  - `s0-snapshot-pointer.json`
- Result:
  - OK pointer `status=ok`
  - needs-manual pointer `status=needs_manual`

### 12.3 core
- Output file: `core_extract.json`
- OK run: `status=ok`, `title/description/currency/price` all non-empty.
- Needs-manual run: `status=needs_manual`, `notes` non-empty.
- Incomplete input path tested: returns `status=error` with non-empty `error`.

### 12.4 images
- Output file: `images_extract.json`
- OK run: `status=ok`, 10 image URLs, all `http/https`, deduped, <= 20.
- Needs-manual run: `status=ok`, `images=[]`.

### 12.5 variations
- Output file: `variations_extract.json`
- OK run: `status=ok`, 20 rows, each row has `title/position/price`.
- Price enrichment uses variation snapshot capture metadata (`_variation<position>_capture.json`) to map `skuId` when available.
- Needs-manual run: `status=ok`, `variations=[]`.

### 12.6 variation image map
- Output file: `variation_image_map_extract.json`
- OK run: `status=ok`, 20 rows, each row has `title/position/images[]`, URLs are `http/https`.
- Needs-manual run: `status=ok`, `variations=[]`.

### 12.7 orchestrator pipeline
- Output files:
  - `_pipeline-state.json`
  - `meta.json`
  - `final.json`
- Helper script:
  - `python3 codex/.codex/skills/taobao-orchestrator-pipeline/scripts/run_pipeline.py --artifact-dir out/artifacts/<run_id>`
- OK run result (`out/artifacts/taobao-s0-ok-20260220-1332`):
  - final `status=ok`
  - stage order completed: S0 -> A -> B -> C -> D -> final_merge
  - `final.json` content matches stdout JSON (byte-level content; trailing newline difference only)
- Needs-manual run result (`out/artifacts/taobao-s0-needsmanual-20260220-1332`):
  - final `status=needs_manual`
  - pipeline gated at S0 (`snapshot_capture=needs_manual`), downstream stages remain pending

## Important Notes for Next Agent
- Do not split checklist into another file; keep tracking in PRD.
- Keep output contracts aligned with Shopee conventions.
- Keep phase-1 scope for variation handling (`颜色分类` only) unless user asks to expand.
- For mapping/enrichment logic, prefer deterministic sources:
  - titles/order from `variations_extract.json`
  - sku/image data from Taobao SSR payload in `s0-initial`
  - snapshot capture metadata for `skuId` mapping (`_variation<position>_capture.json`)
- `docs/taobao_agent_handoff.md` is intentionally uncommitted in current working tree to allow further updates before next handoff commit.

## Next Step
- Start `12.8 Cross-Stage Regression`:
  - rerun regression on 3 URL categories (normal/complex/restricted)
  - verify new orchestrator does not break previously validated stage contracts
  - complete checklist items under `12.8` and then `12.9`
