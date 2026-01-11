package fx

import (
	"go.uber.org/fx"

	"peasydeal-product-miner/internal/server"
)

var Module = fx.Options(
	fx.Provide(server.NewHTTPServer),
	fx.Invoke(RegisterHTTPServerLifecycle),
)
