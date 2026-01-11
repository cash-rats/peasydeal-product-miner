package fx

import (
	"peasydeal-product-miner/cache"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"redis",
	fx.Provide(cache.NewRedis),
)
