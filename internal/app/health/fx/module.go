package fx

import (
	"go.uber.org/fx"

	"peasydeal-product-miner/internal/app/health"
	"peasydeal-product-miner/internal/router"
)

var Module = fx.Options(
	fx.Provide(router.AsRoute(health.NewHandler)),
)
