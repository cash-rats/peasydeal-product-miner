package fx

import "go.uber.org/fx"

var Module = fx.Options(
	CoreRouterOptions,
)
