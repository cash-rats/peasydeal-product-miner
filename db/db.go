package db

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jmoiron/sqlx"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"peasydeal-product-miner/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewSQLXPostgresDB(lc fx.Lifecycle, cfg config.Config, log *zap.SugaredLogger) (*sqlx.DB, error) {
	if strings.TrimSpace(cfg.DBHost) == "" || strings.TrimSpace(cfg.DBName) == "" {
		log.Infow("postgres disabled (missing DB_HOST/DB_NAME)")
		return nil, nil
	}

	db, err := sqlx.Open("pgx", postgresDSN(cfg))
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := db.PingContext(ctx); err != nil {
				_ = db.Close()
				return fmt.Errorf("postgres ping failed: %w", err)
			}
			log.Infow("postgres connected", "host", cfg.DBHost, "db", cfg.DBName)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := db.Close(); err != nil {
				log.Warnw("postgres close failed", "err", err)
			}
			return nil
		},
	})

	return db, nil
}

func postgresDSN(cfg config.Config) string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", cfg.DBHost, cfg.DBPort),
		Path:   cfg.DBName,
	}
	if strings.TrimSpace(cfg.DBUser) != "" {
		if cfg.DBPassword == "" {
			u.User = url.User(cfg.DBUser)
		} else {
			u.User = url.UserPassword(cfg.DBUser, cfg.DBPassword)
		}
	}
	return u.String()
}
