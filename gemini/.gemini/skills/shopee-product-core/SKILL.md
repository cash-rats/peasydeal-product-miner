---
name: shopee-product-core
description: Parse snapshot artifacts offline and return core product fields (title, description, currency, price) as short JSON.
---

# Shopee Product Core (core_extract)

This skill is offline-only.

## Input

Runtime prompt must provide either:

- `artifact_dir` (preferred), or
- explicit paths for `s0-page_state.json` (and optional `s0-snapshot-pointer.json`)

Do not open/navigate browser pages in this skill.
Do not parse HTML artifacts in this skill.

## Required behavior

1. Read `s0-page_state.json` from artifacts (fallback: `page_state.json` for backward compatibility).
2. Extract core fields from `extracted.{title,description,currency,price}` first.
3. Fallback to top-level keys if `extracted` is missing.
4. Enforce `description` max length = 1500 chars.
5. If snapshot indicates blocked/wall, return `status="needs_manual"`.
6. If core fields are incomplete, return `status="error"`.

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
