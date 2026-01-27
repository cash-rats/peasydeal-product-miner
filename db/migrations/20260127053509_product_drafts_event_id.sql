-- +goose Up
-- +goose StatementBegin
-- Add a stable idempotency key for external/workflow events (eg RabbitMQ message_id).
ALTER TABLE product_drafts
ADD COLUMN event_id TEXT;

-- Dedupe by event_id. SQLite UNIQUE indexes allow multiple NULLs.
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_drafts_event_id
  ON product_drafts(event_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_product_drafts_event_id;

-- Note: we intentionally do not DROP COLUMN event_id here.
-- SQLite/Turso support for DROP COLUMN depends on the engine version and
-- dropping columns is not generally reversible without table rebuild.
-- +goose StatementEnd
