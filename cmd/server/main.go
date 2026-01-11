package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	appfx "peasydeal-product-miner/internal/app/fx"
	healthfx "peasydeal-product-miner/internal/app/health/fx"
	routerfx "peasydeal-product-miner/internal/router/fx"
	serverfx "peasydeal-product-miner/internal/server/fx"
)

func main() {
	app := fx.New(
		appfx.Module,
		routerfx.Module,
		serverfx.Module,
		healthfx.Module,
		fx.WithLogger(func(l *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: l}
		}),
	)

	startCtx, cancelStart := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "fx start failed:", err)
		os.Exit(1)
	}

	<-app.Done()

	stopCtx, cancelStop := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStop()
	if err := app.Stop(stopCtx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "fx stop failed:", err)
		os.Exit(1)
	}
}
