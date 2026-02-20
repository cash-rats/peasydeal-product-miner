---
name: taobao-page-snapshot
description: Create HTML-only snapshot artifacts for a Taobao product page using Chrome DevTools MCP interactions + a Python CDP HTML capture helper.
---

# Taobao Page Snapshot (S0, HTML-Only)

You are controlling an already-running Google Chrome via Chrome DevTools.

Goal: perform exactly one browsing session, do required interactions, then use Python script `scripts/cdp_snapshot_html.py` to capture raw HTML artifacts.

S0 does not do structured product extraction. It only writes snapshot artifacts and returns a small JSON pointer.

## Artifact contract

Write files under `out/artifacts/<run_id>/`:

- `s0-initial.html.gz`
- `s0-overlay.html.gz` (best-effort)
- `s0-variation-<position>.html.gz` for first 20 options (best-effort)
- `s0-manifest.json` (small metadata JSON)
- `s0-snapshot-pointer.json` (same JSON object printed to stdout)

Rules:

- Do not print raw HTML in stdout.
- If one snapshot fails, continue best-effort and record notes.
- `run_id` and `artifact_dir` must be non-empty.

## Required Python helper usage

After each interaction state is ready, call:

```bash
python3 ./scripts/cdp_snapshot_html.py \
  --browser-url http://127.0.0.1:9222 \
  --output out/artifacts/<run_id>/<file>.html.gz \
  --url-contains "taobao"
```

Notes:

- Prefer selecting the active Taobao tab via `--url-contains`.
- If needed, use `--target-id` when the target is known.
- Script prints small JSON (status/bytes/hash/truncated). Use it to build manifest entries.

## Output (stdout JSON ONLY)

Return exactly one JSON object:

```json
{
  "url": "string",
  "status": "ok|needs_manual|error",
  "captured_at": "ISO-8601 UTC timestamp",
  "run_id": "string",
  "artifact_dir": "string",
  "snapshot_files": {
    "snapshot": "string",
    "manifest": "string",
    "initial_html": "string",
    "overlay_html": "string",
    "variation_html_dir": "string"
  },
  "tab_tracking": {
    "created_tab": {
      "page_idx": 0,
      "target_id": "string",
      "url": "string"
    },
    "close_attempted": true,
    "close_succeeded": true,
    "close_error": "string"
  },
  "notes": "string",
  "error": "string"
}
```

Rules:

- JSON only. No markdown fences.
- Always include all `snapshot_files` keys. Missing file => `""` and explain in `notes`.
- Always include `tab_tracking` with real values (no placeholders).
- If blocked by login/verification/CAPTCHA and core product content is not visible: `status="needs_manual"`.

## Interaction policy (critical)

Snapshots must be useful for downstream extraction (core/images/variations/variation-image-map), not random captures.

Required signals before capture:

- Core-ready signal: page title + visible price text (`￥`/`¥`) + visible SKU labels (`颜色分类`/`规格`/`型号`/`尺码`) when available.
- Wall signal: login/verification text (e.g. `login.taobao.com`, `验证码`, `安全验证`, `请登录`, `captcha`) and missing core-ready signal.

## Steps (single session)

1. Preflight: `list_pages`; if pages exist, reload first page once.
2. Open new tab and navigate to target URL.
3. Wait until ready (readyState complete + product/price signal), max ~5s retry.
4. Detect wall signals in one evaluate script. If wall and no product content => `needs_manual`.
5. Generate `run_id`, ensure `artifact_dir` exists.
6. Capture initial HTML with Python helper to `s0-initial.html.gz`.
7. Best-effort capture gallery/overlay state:
   - try clicking `图集` tab, then capture `s0-overlay.html.gz`.
   - if `图集` is unavailable, keep current state capture as fallback and record note.
8. Capture variation states (best-effort, first 20):
   - phase-1 scope: use only `颜色分类` group.
   - if `颜色分类` is missing, fallback to first available SKU group and record a note.
   - click options from the chosen group one by one (skip disabled options).
   - after each successful selection, capture `s0-variation-<position>.html.gz`.
   - if selection does not change state, still continue and record note for that position.
9. Write `s0-manifest.json` with file metadata from Python helper outputs.
10. Write `s0-snapshot-pointer.json` using the exact final stdout JSON object.
11. Close the created tab.

## Optional helper for manifest/pointer

To generate `s0-manifest.json` + `s0-snapshot-pointer.json` from capture logs:

```bash
python3 ./scripts/build_s0_outputs.py \
  --artifact-dir out/artifacts/<run_id> \
  --url "<target_url>" \
  --status ok \
  --created-page-idx <idx> \
  --created-target-id "<target_id>" \
  --created-url "<target_url>" \
  --close-attempted \
  --close-succeeded
```

## Final output

Return only the final JSON pointer object.
