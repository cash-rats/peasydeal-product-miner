package fx

import (
	"net/http"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var CoreRouterOptions = fx.Options(
	fx.Provide(NewMux),
)

type muxParams struct {
	fx.In

	Cfg      *config.Config
	Logger   *zap.SugaredLogger
	Handlers []router.Handler `group:"handlers"`
}

func NewMux(p muxParams) *chi.Mux {
	r := chi.NewRouter()

	// CORS for admin console + local dev.
	corsEnabled := false
	allowedOrigins := []string{
		"https://peasydeal-admin-console.vercel.app",
	}
	if p.Cfg != nil {
		switch p.Cfg.ENV {
		case config.Dev, config.Test:
			allowedOrigins = append(allowedOrigins,
				"http://localhost:5173",
				"http://127.0.0.1:5173",
			)
			corsEnabled = true
		case config.Production, config.Preview:
			corsEnabled = true
		}
	}
	if corsEnabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(zapRequestLogger(p.Logger))

	if corsEnabled {
		// Ensure OPTIONS preflight requests get a successful response.
		r.Options("/*", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}

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
