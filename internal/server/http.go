package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"peasydeal-product-miner/config"
)

func NewHTTPServer(cfg config.Config, mux *chi.Mux) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.AppPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
