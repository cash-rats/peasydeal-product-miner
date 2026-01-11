package logs

import (
	"peasydeal-product-miner/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(cfg config.Config) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()
	if cfg.Env == "development" {
		zapCfg = zap.NewDevelopmentConfig()
	}

	level := zapcore.InfoLevel
	if err := level.Set(cfg.LogLevel); err != nil {
		return nil, err
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.InitialFields = map[string]any{
		"app": cfg.AppName,
		"env": cfg.Env,
	}

	return zapCfg.Build()
}

func NewSugaredLogger(logger *zap.Logger) *zap.SugaredLogger {
	return logger.Sugar()
}
