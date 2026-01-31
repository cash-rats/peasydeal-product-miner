package productdrafts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	sqlcdb "peasydeal-product-miner/db/sqlc"
	"peasydeal-product-miner/internal/pkg/render"
	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type GetByIDHandler struct {
	sqliteDB *sqlx.DB
	logger   *zap.SugaredLogger
}

type NewGetByIDHandlerParams struct {
	fx.In

	SQLiteDB *sqlx.DB `name:"sqlite" optional:"true"`
	Logger   *zap.SugaredLogger
}

func NewGetByIDHandler(p NewGetByIDHandlerParams) *GetByIDHandler {
	return &GetByIDHandler{
		sqliteDB: p.SQLiteDB,
		logger:   p.Logger,
	}
}

func (h *GetByIDHandler) RegisterRoute(r *chi.Mux) {
	r.Get("/v1/product-drafts/{id}", h.Handle)
}

type getByIDResponse struct {
	ID                 string  `json:"id"`
	Status             string  `json:"status"`
	Draft              any     `json:"draft"`
	Error              *string `json:"error"`
	CreatedBy          *string `json:"created_by"`
	CreatedAtMs        int64   `json:"created_at_ms"`
	UpdatedAtMs        int64   `json:"updated_at_ms"`
	PublishedAtMs      *int64  `json:"published_at_ms"`
	PublishedProductID *string `json:"published_product_id"`
}

func (h *GetByIDHandler) Handle(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		render.ChiErr(w, http.StatusBadRequest, "missing id")
		return
	}

	if h.sqliteDB == nil {
		render.ChiErr(w, http.StatusServiceUnavailable, "sqlite disabled")
		return
	}

	row, err := sqlcdb.New(h.sqliteDB).GetProductDraft(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		render.ChiErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		h.logger.Errorw("product_draft_get_by_id_failed", "id", id, "err", err)
		render.ChiErr(w, http.StatusInternalServerError, "failed to fetch product draft")
		return
	}

	var draft any
	if err := json.Unmarshal([]byte(row.DraftPayload), &draft); err != nil {
		h.logger.Errorw(
			"product_draft_payload_unmarshal_failed",
			"id", id,
			"err", err,
		)
		render.ChiErr(w, http.StatusInternalServerError, "invalid draft payload")
		return
	}

	resp := getByIDResponse{
		ID:                 row.ID,
		Status:             row.Status,
		Draft:              draft,
		Error:              anyToStringPtr(row.Error),
		CreatedBy:          anyToStringPtr(row.CreatedBy),
		CreatedAtMs:        row.CreatedAtMs,
		UpdatedAtMs:        row.UpdatedAtMs,
		PublishedAtMs:      anyToInt64Ptr(row.PublishedAtMs),
		PublishedProductID: anyToStringPtr(row.PublishedProductID),
	}

	render.ChiJSON(w, http.StatusOK, resp)
}

func anyToStringPtr(v any) *string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		return nullableStringPtr(t)
	case []byte:
		return nullableStringPtr(string(t))
	default:
		return nullableStringPtr(fmt.Sprint(v))
	}
}

func anyToInt64Ptr(v any) *int64 {
	switch t := v.(type) {
	case nil:
		return nil
	case int64:
		return &t
	case int:
		x := int64(t)
		return &x
	case float64:
		x := int64(t)
		return &x
	case []byte:
		s := strings.TrimSpace(string(t))
		if s == "" {
			return nil
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return &n
		}
		return nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return &n
		}
		return nil
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			return nil
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return &n
		}
		return nil
	}
}

func nullableStringPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

var _ router.Handler = (*GetByIDHandler)(nil)
