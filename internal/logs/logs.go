package logs

import (
	"peasydeal-product-miner/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(cfg *config.Config) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()
	if cfg.ENV == config.Dev {
		zapCfg = zap.NewDevelopmentConfig()
	}

	level := zapcore.InfoLevel
	if cfg.ENV == config.Dev {
		level = zapcore.DebugLevel
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.InitialFields = map[string]any{
		"env": cfg.ENV,
	}

	return zapCfg.Build()
}

func NewSugaredLogger(logger *zap.Logger) *zap.SugaredLogger {
	return logger.Sugar()
}
