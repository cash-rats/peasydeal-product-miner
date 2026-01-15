package inngest

import (
	"net/http"
	"strings"

	"peasydeal-product-miner/config"
	pkginngest "peasydeal-product-miner/internal/pkg/inngest"
	"peasydeal-product-miner/internal/pkg/render"
	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/inngest/inngestgo"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type InngestHandler struct {
	logger *zap.SugaredLogger
	cfg    *config.Config
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
		cfg:    p.Config,
		client: p.Client,
	}

}

func (h *InngestHandler) RegisterRoute(r *chi.Mux) {
	path := pkginngest.DefaultServePath
	if h.cfg != nil {
		if v := strings.TrimSpace(h.cfg.Inngest.ServePath); v != "" {
			path = v
		}
	}

	r.Post(path, h.Handle)
	r.Put(path, h.Handle)
	r.Get(path, h.Handle)
}

func (h *InngestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if h.cfg != nil && strings.TrimSpace(h.cfg.Inngest.AppID) == "" {
		render.ChiErr(w, http.StatusNotImplemented, "inngest disabled: set INNGEST_APP_ID to enable")
		return
	}

	h.client.Serve().ServeHTTP(w, r)
}

var _ router.Handler = (*InngestHandler)(nil)
