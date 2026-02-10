---
name: shopee-variation-image-map
description: Parse snapshot HTML artifacts offline and return short JSON mapping variation options to image URLs.
---

# Shopee Variation Image Map (variation_image_map_extract)

This skill is offline-only.

## Input

Runtime prompt must provide:

- `artifact_dir` (preferred)

Do not open/navigate browser pages in this skill.

## Required behavior

1. Read mapping sources from snapshot HTML artifacts:
   - `s0-variation-<position>.html.gz` / `.html` (first 10, best-effort)
   - fallback titles from `variations_extract.json` when selected marker is missing
2. Normalize each item to:
   - `title` (non-empty string)
   - `position` (0-based integer)
   - `images` (array of HTTP/HTTPS URLs)
3. Enforce hard max = 10 mapped options.
4. Per-item failures must be skipped (best-effort behavior).
5. If nothing is found, return `status="ok"` with empty list.
6. Write the same output JSON to `variation_image_map_extract.json` under `artifact_dir` (or explicit `--output` path).

## Required helper usage

Use local helper script in this skill directory:

```bash
python3 ./scripts/extract_variation_image_map_from_html.py --artifact-dir out/artifacts/<run_id> --output out/artifacts/<run_id>/variation_image_map_extract.json
```

## Output (JSON only)

Return exactly one JSON object:

```json
{
  "status": "ok|error",
  "variations": [{"title": "string", "position": 0, "images": ["string"]}],
  "error": "string"
}
```

Rules:

- `variations` must always exist (use `[]` when empty).
- Each variation item must include `images` (use `[]` when empty).
- `status=error` only for unrecoverable artifact read/parse failures.
- Item-level mapping failures must not force `status=error`.
- Output must be JSON only. No markdown fences, no extra text.
