package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"peasydeal-product-miner/db"
	"peasydeal-product-miner/internal/runner"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type ProductDraftStore struct {
	conn      db.Conn
	logger    *zap.SugaredLogger
	validator *validator.Validate
}

type NewProductDraftStoreParams struct {
	fx.In

	Conn   db.Conn `name:"sqlite"`
	Logger *zap.SugaredLogger
}

func NewProductDraftStore(p NewProductDraftStoreParams) *ProductDraftStore {
	return &ProductDraftStore{
		conn:      p.Conn,
		logger:    p.Logger,
		validator: validator.New(),
	}
}

type UpsertFromCrawlResultInput struct {
	EventID   string
	CreatedBy string
	URL       string `validate:"required"`
	Result    runner.Result
}

func (s *ProductDraftStore) UpsertFromCrawlResult(ctx context.Context, in UpsertFromCrawlResultInput) (draftID string, err error) {
	_ = ctx

	if err := s.validator.Struct(in); err != nil {
		return "", fmt.Errorf("validate upsert input: %w", err)
	}

	eventID := strings.TrimSpace(in.EventID)
	createdBy := strings.TrimSpace(in.CreatedBy)
	if createdBy == "" {
		createdBy = "inngest"
	}

	draftID = uuid.NewString()

	if in.Result == nil {
		in.Result = runner.Result{
			"url":    in.URL,
			"status": "error",
			"error":  "missing runner result",
		}
	}

	if rawURL, _ := in.Result["url"].(string); rawURL == "" {
		in.Result["url"] = in.URL
	}

	payloadBytes, err := json.Marshal(in.Result)
	if err != nil {
		payloadBytes, _ = json.Marshal(runner.Result{
			"url":    in.URL,
			"status": "error",
			"error":  fmt.Sprintf("marshal runner result: %v", err),
		})
	}

	status, errorText := draftStatusAndError(in.Result)

	createdByCol := sql.NullString{String: createdBy, Valid: true}
	errorCol := sql.NullString{}
	if errorText != "" {
		errorCol = sql.NullString{String: errorText, Valid: true}
	}

	// Backward-compatible path: if older deployments used event_id as the primary key,
	// update that row in-place and backfill event_id for future dedupe.
	if eventID != "" {
		qLegacy := s.conn.Rebind(`
UPDATE product_drafts
SET
  event_id = ?,
  status = ?,
  draft_payload = ?,
  error = ?,
  created_by = ?
WHERE id = ?
`)

		res, err := s.conn.Exec(qLegacy, eventID, status, string(payloadBytes), errorCol, createdByCol, eventID)
		if err != nil {
			if errors.Is(err, db.ErrSQLiteDisabled) {
				s.logger.Infow("turso_sqlite_disabled_skip_persist", "reason", err.Error())
				return draftID, nil
			}
			return "", fmt.Errorf("update legacy product_drafts by id: %w", err)
		}

		if rows, _ := res.RowsAffected(); rows > 0 {
			s.logger.Infow(
				"product_draft_upserted_from_crawl",
				"id", eventID,
				"event_id", eventID,
				"status", status,
			)
			return eventID, nil
		}
	}

	q := s.conn.Rebind(`
INSERT INTO product_drafts (
  id,
  event_id,
  status,
  draft_payload,
  error,
  created_by
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
)
ON CONFLICT(event_id) DO UPDATE SET
  status = excluded.status,
  draft_payload = excluded.draft_payload,
  error = excluded.error,
  created_by = excluded.created_by
`)

	if _, err := s.conn.Exec(q, draftID, sql.NullString{String: eventID, Valid: eventID != ""}, status, string(payloadBytes), errorCol, createdByCol); err != nil {
		if errors.Is(err, db.ErrSQLiteDisabled) {
			s.logger.Infow("turso_sqlite_disabled_skip_persist", "reason", err.Error())
			return draftID, nil
		}
		return "", fmt.Errorf("upsert product_drafts: %w", err)
	}

	// If we used event_id for dedupe, return the stable draft id stored in the row.
	if eventID != "" {
		var stableID string
		if err := s.conn.QueryRow(s.conn.Rebind("SELECT id FROM product_drafts WHERE event_id = ?"), eventID).Scan(&stableID); err == nil && stableID != "" {
			draftID = stableID
		}
	}

	s.logger.Infow("product_draft_upserted_from_crawl",
		"id", draftID,
		"event_id", eventID,
		"status", status,
	)

	return draftID, nil
}

func draftStatusAndError(result runner.Result) (status string, errorText string) {
	raw, _ := result["status"].(string)

	switch raw {
	case "ok":
		return "READY_FOR_REVIEW", ""
	case "needs_manual":
		return "FAILED", errorFromNeedsManual(result)
	case "error":
		return "FAILED", errorFromResult(result)
	default:
		return "FAILED", errorFromResult(result)
	}
}

func errorFromNeedsManual(result runner.Result) string {
	if s, ok := result["notes"].(string); ok && s != "" {
		return s
	}
	return "crawler returned status=needs_manual"
}

func errorFromResult(result runner.Result) string {
	if s, ok := result["error"].(string); ok && s != "" {
		return s
	}
	return "crawler returned status=error"
}
