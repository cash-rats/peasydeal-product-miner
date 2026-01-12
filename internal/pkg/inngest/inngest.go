package inngest

import (
	"net/url"

	"peasydeal-product-miner/config"

	"github.com/inngest/inngestgo"
)

func NewInngestClient(cfg *config.Config) (inngestgo.Client, error) {
	c, err := inngestgo.NewClient(
		inngestgo.ClientOpts{
			AppID: cfg.Inngest.AppID,
		},
	)

	scheme := "https"
	if cfg.Inngest.Dev == "1" {
		scheme = "http"
	}

	c.SetURL(&url.URL{
		Scheme: scheme,
		Host:   cfg.Inngest.ServeHost,
		Path:   cfg.Inngest.ServePath,
	})

	if err != nil {
		return nil, err
	}
	return c, nil
}
