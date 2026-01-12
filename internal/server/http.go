package server

import (
	"net"
	"net/http"
	"strings"
	"time"

	"peasydeal-product-miner/config"

	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
)

type httpServerParams struct {
	fx.In

	Config *config.Config
	Router *chi.Mux
}

func NewHTTPServer(p httpServerParams) *http.Server {
	addr := strings.TrimSpace(p.Config.App.Addr)
	port := strings.TrimSpace(p.Config.App.Port)
	if addr == "" {
		addr = "0.0.0.0"
	}
	if port == "" {
		port = "8080"
	}

	return &http.Server{
		Addr:              net.JoinHostPort(addr, port),
		Handler:           p.Router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
