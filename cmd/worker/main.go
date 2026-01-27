package main

import (
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	dbfx "peasydeal-product-miner/db/fx"
	crawlworkerfx "peasydeal-product-miner/internal/app/amqp/crawlworker/fx"
	appfx "peasydeal-product-miner/internal/app/fx"
	productdrafts "peasydeal-product-miner/internal/app/inngest/dao"
	"peasydeal-product-miner/internal/runner"
	runnerfx "peasydeal-product-miner/internal/runner/fx"
)

func main() {
	app := fx.New(
		fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: logger}
		}),
		appfx.CoreAppOptions,
		dbfx.SQLiteModule,
		fx.Provide(
			// Runner wiring (same as Inngest domain).
			runnerfx.NewCodexRunnerConfig,
			runnerfx.NewGeminiRunnerConfig,
			runner.NewRunners,
			runner.NewRunner,

			// Persistence for crawl results.
			productdrafts.NewProductDraftStore,
		),
		runnerfx.AsRunner(runner.NewCodexRunner),
		runnerfx.AsRunner(runner.NewGeminiRunner),
		crawlworkerfx.Module,
	)

	app.Run()
}
