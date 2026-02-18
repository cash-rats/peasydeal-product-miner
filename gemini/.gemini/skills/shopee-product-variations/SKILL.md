---
name: shopee-product-variations
description: Parse snapshot HTML artifacts offline and return short JSON for product variation options (including per-variation price when available).
---

# Shopee Product Variations (variations_extract)

This skill is offline-only.

## Input

Runtime prompt must provide either:

- `artifact_dir` (preferred), or
- explicit path for variation html artifact.

Do not open/navigate browser pages in this skill.

## Required behavior

1. Read HTML artifacts from snapshot stage, preferred order:
   - `s0-initial.html.gz`
   - `s0-initial.html`
   - backward compatibility fallback: `s0-page.html.gz`, `s0-page.html`
2. Use optional variation snapshots as enrichment when present:
   - `s0-variation-<position>.html.gz` / `.html` (best-effort)
3. Normalize each option to:
   - `title` (non-empty string)
   - `position` (0-based integer)
   - `price` (string like `$782` or numeric-like string; empty string when unavailable)
4. Extract per-variation price from snapshot HTML:
   - preferred: matching `s0-variation-<position>.html.gz` / `.html`
   - fallback: selected variation price in `s0-initial.html.gz` / `.html`
   - backward compatibility fallback: `s0-page.html.gz` / `.html`
5. Remove duplicates by `title` (keep first occurrence order).
6. Enforce hard max = 20 variations.
7. If nothing is found, return `status="ok"` with empty list.
8. Write the same output JSON to `variations_extract.json` under `artifact_dir` (or explicit `--output` path).

## Required helper usage

Use local helper script in this skill directory:

```bash
python3 ./scripts/extract_variations_from_html.py --artifact-dir out/artifacts/<run_id> --output out/artifacts/<run_id>/variations_extract.json
```

Alternative (explicit file):

```bash
python3 ./scripts/extract_variations_from_html.py --html-path out/artifacts/<run_id>/s0-initial.html.gz --output out/artifacts/<run_id>/variations_extract.json
```

## Output (JSON only)

Return exactly one JSON object:

```json
{
  "status": "ok|error",
  "variations": [{"title": "string", "position": 0, "price": "string"}],
  "error": "string"
}
```

Rules:

- `variations` must always exist (use `[]` when empty).
- Each variation item must include `price` (use `""` when unavailable).
- `status=error` only for unrecoverable artifact read/parse failures.
- Output must be JSON only. No markdown fences, no extra text.
