---
name: shopee-product-images
description: Parse snapshot HTML artifacts offline and return a short JSON list of product image URLs.
---

# Shopee Product Images (images_extract)

This skill is offline-only.

## Input

Runtime prompt must provide either:

- `artifact_dir` (preferred), or
- explicit path for image html artifact.

Do not open/navigate browser pages in this skill.

## Required behavior

1. Read HTML artifacts from snapshot stage, preferred order:
   - `s0-overlay.html.gz`
   - `s0-overlay.html`
   - fallback: `s0-initial.html.gz`, `s0-initial.html`
   - backward compatibility fallback: `s0-page.html.gz`, `s0-page.html`
2. Extract product image URLs from HTML.
3. Keep only HTTP/HTTPS URLs.
4. Deduplicate URLs.
5. Enforce hard max = 20 images.
6. If nothing is found, return `status="ok"` with empty list.
7. Write the same output JSON to `images_extract.json` under `artifact_dir` (or explicit `--output` path).

## Required helper usage

Use local helper script in this skill directory:

```bash
python3 ./scripts/extract_images_from_html.py --artifact-dir out/artifacts/<run_id> --output out/artifacts/<run_id>/images_extract.json
```

Alternative (explicit file):

```bash
python3 ./scripts/extract_images_from_html.py --html-path out/artifacts/<run_id>/s0-overlay.html.gz --output out/artifacts/<run_id>/images_extract.json
```

## Output (JSON only)

Return exactly one JSON object:

```json
{
  "status": "ok|error",
  "images": ["string"],
  "error": "string"
}
```

Rules:

- `images` must always exist (use `[]` when empty).
- `status=error` only for unrecoverable artifact read/parse failures.
- Output must be JSON only. No markdown fences, no extra text.
