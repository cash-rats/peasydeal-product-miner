package fx

import (
	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/app/inngest"
	"peasydeal-product-miner/internal/app/inngest/crawl"
	pkginngest "peasydeal-product-miner/internal/pkg/inngest"
	"peasydeal-product-miner/internal/router"

	"github.com/inngest/inngestgo"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Options(
	fx.Provide(
		pkginngest.NewInngestClient,
		crawl.NewCrawlFunction,
	),
	fx.Invoke(registerFunctions),
	router.AsRoute(inngest.NewInngestHandler),
)

func registerFunctions(
	cfg *config.Config,
	client inngestgo.Client,
	crawlFunc *crawl.CrawlFunction,
	logger *zap.SugaredLogger,
) error {
	_, err := inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "crawl-url"},
		inngestgo.EventTrigger(crawl.CrawlRequestedEventName, nil),
		crawlFunc.Handle,
	)
	if err != nil {
		logger.Errorw(
			"❌ failed to create inngest crawl function",
			"err", err.Error(),
		)
		return err
	}

	logger.Infow("inngest_enabled",
		"path", cfg.Inngest.ServePath,
		"event", crawl.CrawlRequestedEventName,
	)
	return nil
}
