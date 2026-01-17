package fx

import (
	runnerPkg "peasydeal-product-miner/internal/runner"

	"go.uber.org/fx"
)

func AsRunner(f any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			f,
			fx.As(new(runnerPkg.ToolRunner)),
			fx.ResultTags(`group:"tool_runners"`),
		),
	)
}
