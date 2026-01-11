package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"peasydeal-product-miner/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type NewRedisCacheParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    config.Config
	Logger    *zap.SugaredLogger
}

func NewRedis(p NewRedisCacheParams) (*redis.Client, error) {
	if strings.TrimSpace(p.Config.Redis.Host) == "" {
		p.Logger.Info("redis_disabled")
		return nil, nil
	}

	scheme := strings.TrimSpace(p.Config.Redis.Scheme)
	if scheme == "" {
		scheme = "redis"
	}

	var connstr string
	if strings.TrimSpace(p.Config.Redis.User) == "" && strings.TrimSpace(p.Config.Redis.Password) == "" {
		connstr = fmt.Sprintf("%s://%s:%d", scheme, p.Config.Redis.Host, p.Config.Redis.Port)
	} else {
		connstr = fmt.Sprintf(
			"%s://%s:%s@%s:%d",
			scheme,
			p.Config.Redis.User,
			p.Config.Redis.Password,
			p.Config.Redis.Host,
			p.Config.Redis.Port,
		)
	}

	opt, err := redis.ParseURL(connstr)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	opt.PoolSize = 10
	opt.MinIdleConns = 1
	opt.MaxIdleConns = 5
	opt.ConnMaxIdleTime = 5 * time.Minute
	opt.ConnMaxLifetime = 30 * time.Minute

	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	opt.MaxRetries = 3
	opt.MinRetryBackoff = 8 * time.Millisecond
	opt.MaxRetryBackoff = 512 * time.Millisecond

	client := redis.NewClient(opt)

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			return client.Ping(pingCtx).Err()
		},
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})

	p.Logger.Infow("redis_cache_initialized",
		"host", p.Config.Redis.Host,
		"port", p.Config.Redis.Port,
		"user", p.Config.Redis.User,
	)

	return client, nil
}
