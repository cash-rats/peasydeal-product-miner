package fx

import (
	"peasydeal-product-miner/internal/app/amqp/productdrafts"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"amqp-productdrafts",
	fx.Provide(productdrafts.NewProductDraftStore),
)
