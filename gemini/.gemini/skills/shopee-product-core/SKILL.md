---
name: shopee-product-core
description: Parse snapshot HTML artifacts offline and return core product fields (title, description, currency, price) as short JSON.
---

# Shopee Product Core (core_extract)

This skill is offline-only.

## Input

Runtime prompt must provide either:

- `artifact_dir` (preferred), or
- explicit path for HTML artifact: `s0-initial.html.gz` / `s0-initial.html`.

Do not open/navigate browser pages in this skill.

## Required behavior

1. Read HTML artifacts from snapshot stage, preferred order:
   - `s0-initial.html.gz`
   - `s0-initial.html`
   - backward compatibility fallback: `s0-page.html.gz`, `s0-page.html`
2. Parse core fields from HTML/JSON-LD:
   - `title`
   - `description` (max 1500 chars)
   - `currency`
   - `price`
3. If `s0-manifest.json` indicates blocked/wall (or wall keywords are detected in html), return `status="needs_manual"`.
4. If core fields are incomplete, return `status="error"`.

## Required helper usage

Use local helper script in this skill directory:

```bash
python3 ./scripts/extract_core_from_html.py --artifact-dir out/artifacts/<run_id> --output out/artifacts/<run_id>/core_extract.json
```

Alternative (explicit file):

```bash
python3 ./scripts/extract_core_from_html.py --html-path out/artifacts/<run_id>/s0-initial.html.gz
```

The script already handles `.html.gz` and `.html` inputs.

## Output (JSON only)

Return exactly one JSON object:

```json
{
  "status": "ok|needs_manual|error",
  "title": "string",
  "description": "string",
  "currency": "string",
  "price": "number|string",
  "notes": "string",
  "error": "string"
}
```

Rules:

- `status=ok`: must include non-empty `title`, `description`, `currency`, `price`.
- `status=needs_manual`: include non-empty `notes`.
- `status=error`: include non-empty `error`.
- Output must be JSON only. No markdown fences, no extra text.
