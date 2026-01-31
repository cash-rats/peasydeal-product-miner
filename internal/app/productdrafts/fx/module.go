package fx

import (
	"peasydeal-product-miner/internal/app/productdrafts"
	"peasydeal-product-miner/internal/router"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"productdrafts",
	router.AsRoute(productdrafts.NewGetByIDHandler),
)
