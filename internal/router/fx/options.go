package fx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"peasydeal-product-miner/internal/router"
)

type coreRouterParams struct {
	fx.In

	Handlers []router.Handler `group:"handlers"`
	Logger   *zap.SugaredLogger
}

func NewMux(p coreRouterParams) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.NoCache)

	if p.Logger != nil {
		r.Use(requestLogMiddleware(p.Logger))
	}

	for _, h := range p.Handlers {
		if h != nil {
			h.RegisterRoute(r)
		}
	}

	return r
}

func requestLogMiddleware(log *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Infow(
				"http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"size", ww.BytesWritten(),
				"remote", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		})
	}
}
