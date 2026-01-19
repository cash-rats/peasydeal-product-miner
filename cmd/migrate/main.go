package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/db/migrations"
	appfx "peasydeal-product-miner/internal/app/fx"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type MigrateCmd string

func main() {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	app := fx.New(
		fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: logger}
		}),
		appfx.CoreAppOptions,
		fx.Supply(MigrateCmd(cmd)),
		fx.Invoke(registerMigrateHook),
	)

	startCtx, startCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

type migrateHookParams struct {
	fx.In

	Lc     fx.Lifecycle
	Cfg    *config.Config
	Logger *zap.SugaredLogger

	Cmd MigrateCmd
}

func registerMigrateHook(p migrateHookParams) {
	p.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := goose.SetDialect("sqlite3"); err != nil {
				return fmt.Errorf("set goose dialect: %w", err)
			}

			goose.SetBaseFS(migrations.FS)

			dsn := tursoDSNFromConfig(p.Cfg)
			if strings.TrimSpace(dsn) == "" {
				return errors.New("sqlite disabled: set TURSO_SQLITE_DSN and TURSO_SQLITE_TOKEN")
			}

			sqliteDB, err := sqlx.Open("libsql", dsn)
			if err != nil {
				return fmt.Errorf("open sqlite: %w", err)
			}
			defer func() {
				_ = sqliteDB.Close()
			}()

			pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
			defer pingCancel()
			if err := sqliteDB.PingContext(pingCtx); err != nil {
				return fmt.Errorf("ping sqlite: %w", err)
			}
			var one int
			if err := sqliteDB.QueryRowContext(pingCtx, "select 1").Scan(&one); err != nil {
				return fmt.Errorf("pong sqlite: %w", err)
			}
			p.Logger.Infow("✅ sqlite connection ok", tursoDSNLogFields(dsn)...)

			p.Logger.Infow("goose_run_start", "cmd", string(p.Cmd))
			if err := goose.RunContext(ctx, string(p.Cmd), sqliteDB.DB, "."); err != nil {
				return fmt.Errorf("goose run %q: %w", p.Cmd, err)
			}
			p.Logger.Infow("goose_run_done", "cmd", string(p.Cmd))
			return nil
		},
	})
}

func tursoDSNFromConfig(cfg *config.Config) string {
	dsn := strings.TrimSpace(cfg.Turso.DSN)
	if dsn == "" {
		dsn = strings.TrimSpace(cfg.Turso.Path)
	}
	return ensureAuthTokenQuery(dsn, strings.TrimSpace(cfg.Turso.Token))
}

func ensureAuthTokenQuery(dsn, token string) string {
	if token == "" {
		return dsn
	}

	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return dsn
	}

	if strings.EqualFold(u.Scheme, "file") || strings.EqualFold(u.Scheme, "sqlite") {
		return dsn
	}

	q := u.Query()
	if q.Get("authToken") != "" {
		return dsn
	}

	q.Set("authToken", token)
	u.RawQuery = q.Encode()
	return u.String()
}

func tursoDSNLogFields(dsn string) []any {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return []any{"dsn", "unparseable"}
	}
	return []any{"scheme", u.Scheme, "host", u.Host}
}
