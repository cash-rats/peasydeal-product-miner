package fx

import (
	"peasydeal-product-miner/internal/app/health"
	"peasydeal-product-miner/internal/router"

	"go.uber.org/fx"
)

var Module = fx.Options(
	router.AsRoute(health.NewHealthHandler),
)
