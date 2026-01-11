package server

import (
	"fmt"
	"net/http"
	"time"

	"peasydeal-product-miner/config"

	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
)

type httpServerParams struct {
	fx.In

	Config config.Config
	Router *chi.Mux
}

func NewHTTPServer(p httpServerParams) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", p.Config.Port),
		Handler:           p.Router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
