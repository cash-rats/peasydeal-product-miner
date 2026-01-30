package fx

import (
	"peasydeal-product-miner/internal/app/amqp/enqueue"
	"peasydeal-product-miner/internal/pkg/amqpclient"
	"peasydeal-product-miner/internal/router"

	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(
		amqpclient.NewAMQP,
	),
	router.AsRoute(enqueue.NewHandler),
)

