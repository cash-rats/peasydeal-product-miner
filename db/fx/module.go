package fx

import (
	"peasydeal-product-miner/db"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"sqlx-postgres-db",
	fx.Provide(db.NewSQLXPostgresDB),
)
