package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"peasydeal-product-miner/config"
)

func NewRedis(lc fx.Lifecycle, cfg config.Config, log *zap.SugaredLogger) (*redis.Client, error) {
	if strings.TrimSpace(cfg.RedisHost) == "" {
		log.Infow("redis disabled (missing REDIS_HOST)")
		return nil, nil
	}

	addr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	opts := &redis.Options{
		Addr:     addr,
		Username: strings.TrimSpace(cfg.RedisUser),
		Password: cfg.RedisPassword,
	}
	if strings.EqualFold(strings.TrimSpace(cfg.RedisScheme), "rediss") {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	client := redis.NewClient(opts)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := client.Ping(ctx).Err(); err != nil {
				_ = client.Close()
				return fmt.Errorf("redis ping failed: %w", err)
			}
			log.Infow("redis connected", "addr", addr)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := client.Close(); err != nil {
				log.Warnw("redis close failed", "err", err)
			}
			return nil
		},
	})

	return client, nil
}
