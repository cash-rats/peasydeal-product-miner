---
name: shopee-product-images
description: Parse snapshot artifacts offline and return a short JSON list of product image URLs.
---

# Shopee Product Images (images_extract)

This skill is offline-only.

## Input

Runtime prompt must provide either:

- `artifact_dir` (preferred), or
- explicit path(s) for image-related snapshot files

Do not open/navigate browser pages in this skill.

## Required behavior

1. Read image artifacts from snapshot stage, preferred order:
   - `s0-overlay_images.json` (fallback: `overlay_images.json`)
   - image fields inside `s0-page_state.json` if present
   - fallback parse from `s0-page.html` (fallback: `page.html`) if needed
2. Keep only HTTP/HTTPS URLs.
3. Deduplicate URLs.
4. Enforce hard max = 20 images.
5. If nothing is found, return `status="ok"` with empty list.

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
