package productdrafts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	_ "modernc.org/sqlite"
)

func newTestSQLiteDB(t *testing.T) *sqlx.DB {
	t.Helper()

	db, err := sqlx.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
CREATE TABLE product_drafts (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  draft_payload TEXT NOT NULL,
  error TEXT NULL,
  created_by TEXT NULL,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL,
  published_at_ms INTEGER NULL,
  published_product_id TEXT NULL
);
`)
	require.NoError(t, err)

	return db
}

func TestGetByIDHandler_Success(t *testing.T) {
	t.Parallel()

	db := newTestSQLiteDB(t)

	const payload = `{"captured_at":"2026-01-31T08:34:23.026Z","currency":"TWD","description":"desc","images":["https://example.com/img.png"],"price":"1","source":"shopee","status":"ok","title":"t","url":"https://example.com/p"}`

	_, err := db.Exec(
		`INSERT INTO product_drafts (id,status,draft_payload,error,created_by,created_at_ms,updated_at_ms,published_at_ms,published_product_id)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		"draft_1",
		"READY_FOR_REVIEW",
		payload,
		nil,
		"enqueue",
		int64(10),
		int64(11),
		nil,
		nil,
	)
	require.NoError(t, err)

	h := &GetByIDHandler{sqliteDB: db, logger: zap.NewNop().Sugar()}
	r := chi.NewRouter()
	h.RegisterRoute(r)

	req := httptest.NewRequest(http.MethodGet, "/v1/product-drafts/draft_1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var got struct {
		ID                 string         `json:"id"`
		Status             string         `json:"status"`
		Draft              map[string]any `json:"draft"`
		Error              *string        `json:"error"`
		CreatedBy          *string        `json:"created_by"`
		CreatedAtMs        int64          `json:"created_at_ms"`
		UpdatedAtMs        int64          `json:"updated_at_ms"`
		PublishedAtMs      *int64         `json:"published_at_ms"`
		PublishedProductID *string        `json:"published_product_id"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))

	require.Equal(t, "draft_1", got.ID)
	require.Equal(t, "READY_FOR_REVIEW", got.Status)
	require.Nil(t, got.Error)
	require.NotNil(t, got.CreatedBy)
	require.Equal(t, "enqueue", *got.CreatedBy)
	require.Equal(t, int64(10), got.CreatedAtMs)
	require.Equal(t, int64(11), got.UpdatedAtMs)

	require.Equal(t, "TWD", got.Draft["currency"])
	require.Equal(t, "shopee", got.Draft["source"])
	require.Equal(t, "ok", got.Draft["status"])
	require.Equal(t, "https://example.com/p", got.Draft["url"])
}

func TestGetByIDHandler_NotFound(t *testing.T) {
	t.Parallel()

	db := newTestSQLiteDB(t)

	h := &GetByIDHandler{sqliteDB: db, logger: zap.NewNop().Sugar()}
	r := chi.NewRouter()
	h.RegisterRoute(r)

	req := httptest.NewRequest(http.MethodGet, "/v1/product-drafts/missing", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Contains(t, rr.Body.String(), `"error":"not found"`)
}

func TestGetByIDHandler_SQLiteDisabled(t *testing.T) {
	t.Parallel()

	h := &GetByIDHandler{sqliteDB: nil, logger: zap.NewNop().Sugar()}
	r := chi.NewRouter()
	h.RegisterRoute(r)

	req := httptest.NewRequest(http.MethodGet, "/v1/product-drafts/draft_1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
	require.Contains(t, rr.Body.String(), `"error":"sqlite disabled"`)
}
