---
name: shopee-page-snapshot
description: Create HTML-only snapshot artifacts for a Shopee product page using Chrome DevTools MCP interactions + a Python CDP HTML capture helper.
---

# Shopee Page Snapshot (S0, HTML-Only)

You are controlling an already-running Google Chrome via Chrome DevTools.

Goal: perform exactly one browsing session, do required interactions, then use Python script `scripts/cdp_snapshot_html.py` to capture raw HTML artifacts.

S0 does not do structured product extraction. It only writes snapshot artifacts and returns a small JSON pointer.

## Artifact contract

Write files under `out/artifacts/<run_id>/`:

- `s0-initial.html.gz`
- `s0-overlay.html.gz` (best-effort)
- `s0-variation-<position>.html.gz` for first 10 options (best-effort)
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
  --url-contains "shopee"
```

Notes:

- Prefer selecting the active Shopee tab via `--url-contains`.
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
  "notes": "string",
  "error": "string"
}
```

Rules:

- JSON only. No markdown fences.
- Always include all `snapshot_files` keys. Missing file => `""` and explain in `notes`.
- If blocked by login/verification/CAPTCHA and core product content is not visible: `status="needs_manual"`.

## Steps (single session)

1. Preflight: `list_pages`; if pages exist, reload first page once.
2. Open new tab and navigate to target URL.
3. Wait until ready (readyState complete + product/price signal), max ~5s retry.
4. Detect wall signals in one evaluate script. If wall and no product content => `needs_manual`.
5. Generate `run_id`, ensure `artifact_dir` exists.
6. Capture initial HTML with Python helper to `s0-initial.html.gz`.
7. Best-effort open image overlay; capture `s0-overlay.html.gz`.
8. Find variation options (up to 10). For each option, interact then capture `s0-variation-<position>.html.gz`.
9. Write `s0-manifest.json` with file metadata from Python helper outputs.
10. Write `s0-snapshot-pointer.json` using the exact final stdout JSON object.
11. Close the created tab.

## Final output

Return only the final JSON pointer object.
