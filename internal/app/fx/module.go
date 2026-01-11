package fx

import (
	"go.uber.org/fx"

	"peasydeal-product-miner/cache"
	"peasydeal-product-miner/config"
	"peasydeal-product-miner/db"
	"peasydeal-product-miner/internal/logs"
)

var Module = fx.Options(
	fx.Provide(
		config.NewViper,
		config.NewConfig,
		logs.NewLogger,
		logs.NewSugaredLogger,
		db.NewSQLXPostgresDB,
		cache.NewRedis,
	),
	fx.Invoke(logs.RegisterLifecycle),
)
