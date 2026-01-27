package fx

import (
	"context"

	"peasydeal-product-miner/internal/app/amqp/crawlworker"
	"peasydeal-product-miner/internal/pkg/amqpclient"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module(
	"amqp-crawlworker",
	fx.Provide(
		amqpclient.NewAMQP,
		fx.Annotate(
			crawlworker.NewCrawlHandler,
			fx.As(new(crawlworker.Handler)),
		),
		crawlworker.NewConsumer,
	),
	fx.Invoke(registerLifecycleHooks),
)

type hooksParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Consumer  *crawlworker.Consumer
	Logger    *zap.SugaredLogger
}

func registerLifecycleHooks(p hooksParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			p.Logger.Infow("crawlworker_starting")
			return p.Consumer.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			p.Logger.Infow("crawlworker_stopping")
			return p.Consumer.Stop(ctx)
		},
	})
}
