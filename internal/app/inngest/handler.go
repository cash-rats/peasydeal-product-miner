package inngest

import (
	"net/http"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/inngest/inngestgo"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type InngestHandler struct {
	logger *zap.SugaredLogger
	client inngestgo.Client
}

type NewInngestHandlerParams struct {
	fx.In

	Logger *zap.SugaredLogger
	Config *config.Config
	Client inngestgo.Client
}

func NewInngestHandler(p NewInngestHandlerParams) *InngestHandler {
	return &InngestHandler{
		logger: p.Logger,
		client: p.Client,
	}

}

func (h *InngestHandler) RegisterRoute(r *chi.Mux) {
	r.Post("/api/inngest", h.Handle)
}

func (h *InngestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	h.client.Serve()
}

var _ router.Handler = (*InngestHandler)(nil)
