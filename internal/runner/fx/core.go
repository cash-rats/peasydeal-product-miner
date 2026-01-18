package fx

import (
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

func NewCodexRunnerConfig(logger *zap.SugaredLogger) runnerPkg.CodexRunnerConfig {
	return runnerPkg.CodexRunnerConfig{
		Cmd:              "codex",
		Model:            "gpt-5.2",
		SkipGitRepoCheck: true,
		Logger:           logger,
	}
}

func NewGeminiRunnerConfig(logger *zap.SugaredLogger) runnerPkg.GeminiRunnerConfig {
	return runnerPkg.GeminiRunnerConfig{
		Cmd:    "gemini",
		Model:  "",
		Logger: logger,
	}
}
