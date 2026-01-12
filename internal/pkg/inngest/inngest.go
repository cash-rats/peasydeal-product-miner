package inngest

import (
	"net/url"
	"strings"

	"peasydeal-product-miner/config"

	"github.com/inngest/inngestgo"
)

const DefaultServePath = "/api/inngest"

func NewInngestClient(cfg *config.Config) (inngestgo.Client, error) {
	scheme := "https"
	if cfg.Inngest.Dev == "1" {
		scheme = "http"
	}

	opts := inngestgo.ClientOpts{
		AppID: cfg.Inngest.AppID,
		Dev:   inngestgo.BoolPtr(cfg.Inngest.Dev == "1"),
	}
	signingKey := strings.TrimSpace(cfg.Inngest.SigningKey)
	opts.SigningKey = &signingKey
	c, err := inngestgo.NewClient(opts)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(cfg.Inngest.ServeHost) != "" {
		c.SetURL(&url.URL{
			Scheme: scheme,
			Host:   strings.TrimSpace(cfg.Inngest.ServeHost),
			Path:   cfg.Inngest.ServePath,
		})
	}

	return c, nil
}
