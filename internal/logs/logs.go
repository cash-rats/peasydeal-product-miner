package logs

import (
	"context"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"peasydeal-product-miner/config"
)

func NewLogger(cfg config.Config) (*zap.Logger, error) {
	zcfg := zap.NewProductionConfig()
	zcfg.Level = zap.NewAtomicLevelAt(levelFromString(cfg.LogLevel))
	zcfg.OutputPaths = []string{"stderr"}
	zcfg.ErrorOutputPaths = []string{"stderr"}

	return zcfg.Build()
}

func NewSugaredLogger(l *zap.Logger) *zap.SugaredLogger {
	return l.Sugar()
}

func RegisterLifecycle(lc fx.Lifecycle, l *zap.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			_ = l.Sync()
			return nil
		},
	})
}

func levelFromString(raw string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
