package inngest

import (
	"log"
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

	log.Printf("~~1 %v  %v %v %v", scheme, cfg.Inngest.ServeHost, cfg.Inngest.ServePath, cfg.Inngest.AppID)

	opts := inngestgo.ClientOpts{
		AppID: cfg.Inngest.AppID,
		// Dev:             inngestgo.BoolPtr(cfg.Inngest.Dev == "1"),
		Dev: inngestgo.BoolPtr(true),
		// EventAPIBaseURL: inngestgo.StrPtr("http://localhost:3010/api/inngest"),
	}
	signingKey := strings.TrimSpace(cfg.Inngest.SigningKey)
	opts.SigningKey = &signingKey
	c, err := inngestgo.NewClient(opts)
	if err != nil {
		return nil, err
	}

	c.SetURL(&url.URL{
		Scheme: scheme,
		Host:   strings.TrimSpace(cfg.Inngest.ServeHost),
		Path:   cfg.Inngest.ServePath,
	})

	return c, nil
}
