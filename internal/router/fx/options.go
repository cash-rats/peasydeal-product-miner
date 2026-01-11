package fx

import (
	"net/http"
	"time"

	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var CoreRouterOptions = fx.Options(
	fx.Provide(NewMux),
)

type muxParams struct {
	fx.In

	Logger   *zap.SugaredLogger
	Handlers []router.Handler `group:"routes"`
}

func NewMux(p muxParams) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(zapRequestLogger(p.Logger))

	for _, h := range p.Handlers {
		h.RegisterRoute(r)
	}

	return r
}

func zapRequestLogger(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			next.ServeHTTP(ww, r)

			logger.Infow("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration", time.Since(start),
			)
		})
	}
}
