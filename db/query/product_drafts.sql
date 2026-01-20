-- Product drafts (Turso / SQLite)

-- name: CreateProductDraft :exec
INSERT INTO product_drafts (
  id,
  status,
  draft_payload,
  error,
  created_by
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?
);

-- name: GetProductDraft :one
SELECT
  id,
  status,
  draft_payload,
  error,
  created_by,
  created_at_ms,
  updated_at_ms,
  published_at_ms,
  published_product_id
FROM product_drafts
WHERE id = ?;

-- name: ListProductDraftsByStatus :many
SELECT
  id,
  status,
  url,
  source,
  title,
  currency,
  price,
  error,
  created_by,
  created_at_ms,
  updated_at_ms,
  published_at_ms,
  published_product_id
FROM product_drafts
WHERE status = ?
ORDER BY updated_at_ms DESC
LIMIT ?;

