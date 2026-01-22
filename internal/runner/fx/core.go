package fx

import (
	"peasydeal-product-miner/config"
	runnerPkg "peasydeal-product-miner/internal/runner"

	"go.uber.org/fx"
	"go.uber.org/zap"
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

// Provide config struct for `runnerPkg.CodexRunnerConfig`
type NewCodexRunnerConfigParams struct {
	fx.In

	Logger *zap.SugaredLogger
	Cfg    *config.Config
}

func NewCodexRunnerConfig(p NewCodexRunnerConfigParams) runnerPkg.CodexRunnerConfig {
	return runnerPkg.CodexRunnerConfig{
		Cmd:              "codex",
		Model:            p.Cfg.CodexModel,
		SkipGitRepoCheck: true,
		Logger:           p.Logger,
	}
}

type NewGeminiRunnerConfigParams struct {
	fx.In

	Logger *zap.SugaredLogger
	Cfg    *config.Config
}

func NewGeminiRunnerConfig(p NewGeminiRunnerConfigParams) runnerPkg.GeminiRunnerConfig {
	model := ""
	if p.Cfg != nil {
		model = p.Cfg.GeminiModel
	}
	return runnerPkg.GeminiRunnerConfig{
		Cmd:    "gemini",
		Model:  model,
		Logger: p.Logger,
	}
}
