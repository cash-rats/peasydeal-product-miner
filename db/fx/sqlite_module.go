package fx

import (
	"peasydeal-product-miner/db"

	"go.uber.org/fx"
)

var SQLiteModule = fx.Module(
	"sqlx-sqlite-db",
	fx.Provide(db.NewSQLXSQLiteDB),
)
