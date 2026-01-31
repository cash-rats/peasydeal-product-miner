package main

import (
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	cachefx "peasydeal-product-miner/cache/fx"
	dbfx "peasydeal-product-miner/db/fx"
	enqueuefx "peasydeal-product-miner/internal/app/amqp/enqueue/fx"
	productdraftsfx "peasydeal-product-miner/internal/app/amqp/productdrafts/fx"
	appfx "peasydeal-product-miner/internal/app/fx"
	healthfx "peasydeal-product-miner/internal/app/health/fx"
	inngestfx "peasydeal-product-miner/internal/app/inngest/fx"
	productdraftsapifx "peasydeal-product-miner/internal/app/productdrafts/fx"
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
		dbfx.SQLiteModule,
		cachefx.Module,
		routerfx.CoreRouterOptions,
		serverfx.ServerOptions,
		healthfx.Module,
		productdraftsapifx.Module,
		productdraftsfx.Module,
		inngestfx.Module,
		enqueuefx.Module,
	)

	app.Run()
}
