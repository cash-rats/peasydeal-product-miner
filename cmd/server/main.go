package main

import (
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	cachefx "peasydeal-product-miner/cache/fx"
	dbfx "peasydeal-product-miner/db/fx"
	appfx "peasydeal-product-miner/internal/app/fx"
	healthfx "peasydeal-product-miner/internal/app/health/fx"
	inngestfx "peasydeal-product-miner/internal/app/inngest/fx"
	routerfx "peasydeal-product-miner/internal/router/fx"
	serverfx "peasydeal-product-miner/internal/server/fx"
)

func main() {
	app := fx.New(
		fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: logger}
		}),
		appfx.CoreAppOptions,
		dbfx.Module,
		cachefx.Module,
		routerfx.CoreRouterOptions,
		serverfx.ServerOptions,
		healthfx.Module,
		inngestfx.Module,
	)

	app.Run()
}
