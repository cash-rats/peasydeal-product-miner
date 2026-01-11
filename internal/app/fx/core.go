package fx

import (
	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/logs"

	"go.uber.org/fx"
)

var CoreAppOptions = fx.Options(
	fx.Provide(
		config.NewViper,
		config.NewConfig,
		logs.NewLogger,
		logs.NewSugaredLogger,
	),
)
