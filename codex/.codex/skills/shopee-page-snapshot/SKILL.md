---
name: shopee-page-snapshot
description: Create a master snapshot (state + artifacts) for a Shopee product page using Chrome DevTools MCP, then output a small JSON pointer to the artifacts.
---

# Shopee Page Snapshot (S0)

You are controlling an already-running Google Chrome via Chrome DevTools.

Goal: perform **exactly one** browsing session to capture all state needed for downstream offline analysis:
- core signals (title/description/price/currency + wall detection)
- images (overlay thumbnails, limited)
- variations (options text, limited)
- variation -> image mapping for **first 10** options (best-effort; failures skipped)

Important: the final stdout must be **a small JSON object only**. All large data (HTML, JSON-LD, DOM dumps) must be written to files under `out/artifacts/<run_id>/` (repo-relative; works on host and in Docker where `/app/out` maps to `/out`).

## Artifact file contracts (must be valid JSON / real data)

Do NOT write placeholders like `...` or `{...}` or `[...]` into artifact files.
All artifact JSON files must be valid JSON. If something is too large, truncate it and add a `*_truncated=true` flag + length metadata.

Hard rule (must follow):
- When writing `*.json` artifacts, you MUST construct a JavaScript object and write it using `JSON.stringify(obj)` (or equivalent). Never hand-build JSON strings.
- JSON string values MUST NOT contain raw control characters (e.g. literal CR/LF/TAB). If the source text contains line breaks, they must be preserved via JSON escaping (which `JSON.stringify` handles).
- After creating the JSON string, you MUST validate it in-page with `JSON.parse(jsonString)` before writing to disk. If validation fails, set `status="error"` and explain in `error`/`notes` (still write whatever minimal artifacts you safely can).
- `s0-page_state.json` MUST be **minimal** and MUST NOT include non-core debug fields like `meta` or `jsonld_raw`. Those frequently contain multiline text or unescaped quotes and can break JSON. If you want to save them, write separate artifacts (e.g. `meta.json`, `jsonld_raw.txt`) without risking the core pipeline.

`s0-page_state.json` MUST be a JSON object with at least:
```json
{
  "url": "string",
  "captured_at": "ISO-8601 UTC timestamp",
  "href": "string",
  "title": "string",
  "readyState": "string",
  "blocked": false,
  "block_reason": "string",
  "extracted": {
    "title": "string",
    "description": "string (<=1500 chars)",
    "currency": "string (e.g. TWD)",
    "price": "number or numeric string"
  }
}
```

`s0-page.html`: write `document.documentElement.outerHTML` (may be truncated; do not use `...`).

## Output (stdout JSON ONLY)

Return EXACTLY ONE JSON object with this shape:
```json
{
  "url": "string",
  "status": "ok | needs_manual | error",
  "captured_at": "ISO-8601 UTC timestamp",
  "run_id": "string",
  "artifact_dir": "string",
  "snapshot_files": {
    "snapshot": "string",
    "page_html": "string",
    "page_state": "string",
    "overlay_images": "string",
    "variations": "string",
    "variation_image_map": "string"
  },
  "notes": "string",
  "error": "string"
}
```

Rules:
- JSON ONLY. No markdown fences. No extra text.
- Always include `snapshot_files` keys. If a file could not be produced, set its value to `""` and explain in `notes`.
- `run_id` and `artifact_dir` MUST be non-empty. Set `artifact_dir` to `out/artifacts/<run_id>`.
- Standard snapshot filenames under `artifact_dir`:
  - `s0-snapshot-pointer.json`
  - `s0-page_state.json`
  - `s0-page.html`
  - `s0-overlay_images.json`
  - `s0-variations.json`
  - `s0-variation_image_map.json`
- `snapshot_files` values must point to these standard files (relative names are preferred).
- If blocked by login/verification/CAPTCHA: `status="needs_manual"` and explain in `notes` (still write whatever artifacts you can).
- If something is fundamentally broken (cannot navigate / cannot talk to CDP): `status="error"` and explain in `error`.

## Snapshot constraints (hard limits)
- `description` in saved state: max 1500 chars (truncate).
- `images`: max 20 URLs.
- `variations`: max 20 options.
- `variation_image_map`: max 10 options; per-option failure => skip that option and continue.

## Steps (single navigation session)

### A) Preflight crash recovery (Aw, Snap!)
1) DevTools `list_pages`.
2) If there is at least one page:
   - DevTools `select_page` on the first page
   - DevTools `navigate_page` type=`reload` (ignoreCache=true if available)

### B) Open + navigate
3) DevTools `new_page`. Record page index as `pageIdx`.
4) DevTools `navigate_page` to the target URL from the runtime prompt (in the new tab).

### C) Wait until ready
5) Use DevTools `evaluate_script` to check readiness:
   - `document.readyState === "complete"`
   - AND either:
     - JSON-LD Product exists (`script[type="application/ld+json"]` containing Product), OR
     - a price-like string (e.g. `NT$` / `$`) is visible in the body
6) If not ready, wait up to ~5 seconds total (repeat the readiness check a few times). If still not ready, continue but note it in `notes`.

### D) Detect wall early
7) In ONE DevTools `evaluate_script`, detect blocking wall signals:
   - login / verification / CAPTCHA / robot / slider
   - If wall is present AND core product content is not visible, set `status="needs_manual"` and write minimal artifacts.

### E) Capture page state (write artifacts)
8) Generate `run_id` (UTC timestamp + short random suffix).
9) Set `artifact_dir = out/artifacts/<run_id>` and ensure the directory exists (use file tools).
10) In ONE `evaluate_script`, collect a **minimal** `page_state` object and (separately) `outerHTML`:
    - `href`, `title`, `readyState`
    - Core candidates: extracted title/description/price/currency (best-effort; if not blocked but missing, set `status="error"`)
    - `outerHTML` of `document.documentElement.outerHTML` (if extremely large, truncate and record `html_truncated=true`)
    - Optional debug artifacts (NOT inside `s0-page_state.json`):
      - `meta.json`: collect a small map of meta tags (must be valid JSON)
      - `jsonld_raw.txt`: collect each `script[type="application/ld+json"].textContent` as plain text lines (avoid JSON escaping risks)
11) Write:
    - `out/artifacts/<run_id>/s0-page.html`  (HTML string; may be truncated)
    - `out/artifacts/<run_id>/s0-page_state.json` (the compact state object; JSON)
12) Write `out/artifacts/<run_id>/s0-snapshot-pointer.json` containing the **same JSON object** you will print to stdout.

### F) Capture images (overlay modal)
12) Best-effort open the product image overlay:
   - Prefer: click the main product image area.
   - If click is flaky, use `evaluate_script` to locate a likely main image and call `.click()`.
13) In ONE `evaluate_script`, extract image URLs from the overlay dialog DOM:
   - Find `[role="dialog"]` (fallback `[aria-modal="true"]` / `.shopee-modal`)
   - Collect `img` sources (`currentSrc/src/data-src/data-lazy`)
   - Keep only `susercontent.com/file/`
   - Deduplicate and cap to 20
14) Write `out/artifacts/<run_id>/s0-overlay_images.json`

### G) Capture variations (options)
15) In ONE `evaluate_script`, find variation option buttons (規格/款式/顏色/樣式):
   - Extract option text (trim, <= 50 chars)
   - Return up to 20 options in DOM order with 0-based `position`
16) Write `out/artifacts/<run_id>/s0-variations.json`

### H) Variation -> image mapping (first 10, best-effort)
17) For the first 10 variation options:
   - Hover (or click) the option (best-effort)
   - Read the current main image URL (`currentSrc/src/data-src`)
   - If an option fails to map, skip it and continue
18) Write `out/artifacts/<run_id>/s0-variation_image_map.json`

### I) Close tab
19) DevTools `close_page` with `pageIdx` (always).

## Final output
Return the stdout JSON pointer to the artifacts (small, stable, always closed JSON).
