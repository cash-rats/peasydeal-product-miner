package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
	EventID string
	URL     string `validate:"required"`
	Result  runner.Result
}

func (s *ProductDraftStore) UpsertFromCrawlResult(ctx context.Context, in UpsertFromCrawlResultInput) (draftID string, err error) {
	_ = ctx

	if err := s.validator.Struct(in); err != nil {
		return "", fmt.Errorf("validate upsert input: %w", err)
	}

	draftID = in.EventID
	if draftID == "" {
		draftID = uuid.NewString()
	}

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

	createdBy := sql.NullString{String: "inngest", Valid: true}
	errorCol := sql.NullString{}
	if errorText != "" {
		errorCol = sql.NullString{String: errorText, Valid: true}
	}

	q := s.conn.Rebind(`
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
)
ON CONFLICT(id) DO UPDATE SET
  status = excluded.status,
  draft_payload = excluded.draft_payload,
  error = excluded.error,
  created_by = excluded.created_by
`)

	if _, err := s.conn.Exec(q, draftID, status, string(payloadBytes), errorCol, createdBy); err != nil {
		if errors.Is(err, db.ErrSQLiteDisabled) {
			s.logger.Infow(
				"turso_sqlite_disabled_skip_persist",
				"reason", err.Error(),
			)
			return draftID, nil
		}
		return "", fmt.Errorf("upsert product_drafts: %w", err)
	}

	s.logger.Infow(
		"product_draft_upserted_from_crawl",
		"id", draftID,
		"status", status,
	)

	return draftID, nil
}

func draftStatusAndError(result runner.Result) (status string, errorText string) {
	raw, _ := result["status"].(string)

	switch raw {
	case "ok", "needs_manual":
		return "READY_FOR_REVIEW", ""
	case "error":
		return "FAILED", errorFromResult(result)
	default:
		return "FAILED", errorFromResult(result)
	}
}

func errorFromResult(result runner.Result) string {
	if s, ok := result["error"].(string); ok && s != "" {
		return s
	}
	return "crawler returned status=error"
}
