package inngest

import (
	"net/http"
	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
)

type InngestHandler struct{}

func NewInngestHandler() *InngestHandler {
	return &InngestHandler{}

}

func (h *InngestHandler) RegisterRoute(r *chi.Mux) {

}

func (h *InngestHandler) Handle(w http.ResponseWriter, r *http.Request) {

}

var _ router.Handler = (*InngestHandler)(nil)
