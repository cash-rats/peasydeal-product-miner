package inngest

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/pkg/render"

	"github.com/inngest/inngestgo"
)

const DefaultServePath = "/api/inngest"

func NewInngestClient(cfg *config.Config) (inngestgo.Client, error) {
	appID := strings.TrimSpace(cfg.Inngest.AppID)
	if appID == "" {
		return disabledClient{reason: "inngest disabled: set INNGEST_APP_ID to enable"}, nil
	}

	dev := strings.TrimSpace(cfg.Inngest.Dev)
	opts := inngestgo.ClientOpts{
		AppID: appID,
		Dev:   inngestgo.BoolPtr(dev != ""),
	}

	// If cfg.Inngest.Dev is a URL (eg "http://localhost:8288"), force the SDK to use it
	// for API + event endpoints. This avoids relying on process env var INNGEST_DEV which
	// may differ from our Viper-derived config (notably in tests).
	if u, err := url.Parse(dev); err == nil && u.Host != "" {
		base := strings.TrimRight(dev, "/")
		opts.APIBaseURL = &base
		opts.EventAPIBaseURL = &base
	}

	if signingKey := strings.TrimSpace(cfg.Inngest.SigningKey); signingKey != "" {
		opts.SigningKey = &signingKey
	}

	c, err := inngestgo.NewClient(opts)
	if err != nil {
		return nil, err
	}

	if serveHost := strings.TrimSpace(cfg.Inngest.ServeHost); serveHost != "" {
		scheme := "https"
		if dev != "" {
			scheme = "http"
		}

		servePath := strings.TrimSpace(cfg.Inngest.ServePath)
		if servePath == "" {
			servePath = DefaultServePath
		}

		c.SetURL(&url.URL{
			Scheme: scheme,
			Host:   serveHost,
			Path:   servePath,
		})
	}

	return c, nil
}

var errInngestDisabled = errors.New("inngest disabled")

type disabledClient struct {
	reason string
}

func (c disabledClient) AppID() string { return "" }

func (c disabledClient) Send(ctx context.Context, evt any) (string, error) {
	return "", errInngestDisabled
}

func (c disabledClient) SendMany(ctx context.Context, evt []any) ([]string, error) {
	return nil, errInngestDisabled
}

func (c disabledClient) Options() inngestgo.ClientOpts { return inngestgo.ClientOpts{} }

func (c disabledClient) Serve() http.Handler { return c.ServeWithOpts(inngestgo.ServeOpts{}) }

func (c disabledClient) ServeWithOpts(opts inngestgo.ServeOpts) http.Handler {
	msg := strings.TrimSpace(c.reason)
	if msg == "" {
		msg = "inngest disabled"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		render.ChiErr(w, http.StatusNotImplemented, msg)
	})
}

func (c disabledClient) SetOptions(opts inngestgo.ClientOpts) error { return errInngestDisabled }
func (c disabledClient) SetURL(u *url.URL)                          {}
