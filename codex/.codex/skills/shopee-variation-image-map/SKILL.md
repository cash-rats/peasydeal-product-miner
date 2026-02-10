---
name: shopee-variation-image-map
description: Parse snapshot artifacts offline and return short JSON mapping variation options to image URLs.
---

# Shopee Variation Image Map (variation_image_map_extract)

This skill is offline-only.

## Input

Runtime prompt must provide either:

- `artifact_dir` (preferred), or
- explicit path(s) for mapping-related snapshot files

Do not open/navigate browser pages in this skill.

## Required behavior

1. Read mapping artifacts from snapshot stage, preferred order:
   - `s0-variation_image_map.json` (fallback: `variation_image_map.json`)
   - mapping-like data in `s0-page_state.json`
2. Normalize each item to:
   - `title` (non-empty string)
   - `position` (0-based integer)
   - `images` (array of HTTP/HTTPS URLs)
   - Compatibility input: accept legacy single `image` and normalize to `images=[image]`
3. Enforce hard max = 10 mapped options.
4. Per-item failures must be skipped (best-effort behavior).
5. If nothing is found, return `status="ok"` with empty list.

## Output (JSON only)

Return exactly one JSON object:

```json
{
  "status": "ok|error",
  "variations": [{"title": "string", "position": 0, "images": ["string"], "image": "string"}],
  "error": "string"
}
```

Rules:

- `variations` must always exist (use `[]` when empty).
- Each variation item must include `images` (use `[]` when empty).
- `status=error` only for unrecoverable artifact read/parse failures.
- Item-level mapping failures must not force `status=error`.
- During migration, you may include legacy `image` as the first element of `images`.
- Output must be JSON only. No markdown fences, no extra text.
