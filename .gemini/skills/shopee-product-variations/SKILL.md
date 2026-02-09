---
name: shopee-product-variations
description: Parse snapshot artifacts offline and return short JSON for product variation options.
---

# Shopee Product Variations (variations_extract)

This skill is offline-only.

## Input

Runtime prompt must provide either:

- `artifact_dir` (preferred), or
- explicit path(s) for variation-related snapshot files

Do not open/navigate browser pages in this skill.

## Required behavior

1. Read variation artifacts from snapshot stage, preferred order:
   - `s0-variations.json` (fallback: `variations.json`)
   - variation-like data in `s0-page_state.json`
   - fallback parse from `s0-page.html` (fallback: `page.html`) if needed
2. Normalize each option to:
   - `title` (non-empty string)
   - `position` (0-based integer)
3. Remove duplicates by (`title`,`position`).
4. Enforce hard max = 20 variations.
5. If nothing is found, return `status="ok"` with empty list.

## Output (JSON only)

Return exactly one JSON object:

```json
{
  "status": "ok|error",
  "variations": [{"title": "string", "position": 0}],
  "error": "string"
}
```

Rules:

- `variations` must always exist (use `[]` when empty).
- `status=error` only for unrecoverable artifact read/parse failures.
- Output must be JSON only. No markdown fences, no extra text.
