package health

import (
	"net/http"

	"peasydeal-product-miner/internal/pkg/render"

	"github.com/go-chi/chi/v5"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) RegisterRoute(r *chi.Mux) {
	r.Get("/health", h.Handle)
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	render.ChiJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})
}
