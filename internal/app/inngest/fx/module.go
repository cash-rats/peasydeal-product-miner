package fx

import (
	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/app/inngest"
	"peasydeal-product-miner/internal/app/inngest/crawl"
	productdrafts "peasydeal-product-miner/internal/app/inngest/dao"
	pkginngest "peasydeal-product-miner/internal/pkg/inngest"
	"peasydeal-product-miner/internal/router"
	"peasydeal-product-miner/internal/runner"
	runnerFx "peasydeal-product-miner/internal/runner/fx"

	"github.com/inngest/inngestgo"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Options(
	runnerFx.AsRunner(runner.NewCodexRunner),
	runnerFx.AsRunner(runner.NewGeminiRunner),
	fx.Provide(
		runnerFx.NewCodexRunnerConfig,
		runnerFx.NewGeminiRunnerConfig,
		runner.NewRunners,
		runner.NewRunner,
		pkginngest.NewInngestClient,
		productdrafts.NewProductDraftStore,
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
	if cfg != nil && cfg.Inngest.AppID == "" {
		logger.Infow("inngest_disabled", "reason", "missing INNGEST_APP_ID")
		return nil
	}

	_, err := inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{
			ID: "crawl-url",
			// Idempotency: inngestgo.StrPtr("event.id"),
			Retries: inngestgo.IntPtr(0),
		},
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
