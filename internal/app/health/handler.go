package health

import (
	"errors"
	"net/http"

	"peasydeal-product-miner/db"
	"peasydeal-product-miner/internal/pkg/render"
	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
)

type Handler struct {
	sqliteConn db.Conn
}

type NewHealthHandlerParams struct {
	fx.In

	SQLiteConn db.Conn `name:"sqlite" optional:"true"`
}

func NewHealthHandler(p NewHealthHandlerParams) *Handler {
	return &Handler{sqliteConn: p.SQLiteConn}
}

func (h *Handler) RegisterRoute(r *chi.Mux) {
	r.Get("/health", h.Handle)
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	sqliteStatus := map[string]any{
		"enabled": h.sqliteConn != nil,
	}

	if h.sqliteConn != nil {
		var one int
		err := h.sqliteConn.QueryRow("select 1").Scan(&one)
		if errors.Is(err, db.ErrSQLiteDisabled) {
			sqliteStatus["enabled"] = false
			sqliteStatus["status"] = "disabled"
		} else if err != nil {
			render.ChiJSON(w, http.StatusServiceUnavailable, map[string]any{
				"ok":     false,
				"sqlite": map[string]any{"status": "error", "error": err.Error()},
			})
			return
		} else {
			sqliteStatus["status"] = "ok"
		}
	}

	render.ChiJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"sqlite": sqliteStatus,
	})
}

var _ router.Handler = (*Handler)(nil)
