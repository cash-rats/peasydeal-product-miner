package fx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"peasydeal-product-miner/config"

	"go.uber.org/zap"
)

func TestNewMux_CORSPreflight_AllowsLocalhost5173_InDev(t *testing.T) {
	cfg := &config.Config{}
	cfg.ENV = config.Dev

	r := NewMux(muxParams{
		Cfg:      cfg,
		Logger:   zap.NewNop().Sugar(),
		Handlers: nil,
	})

	req := httptest.NewRequest(http.MethodOptions, "/v1/crawl/enqueue", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow-origin=%q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("missing allow-methods")
	}
}
