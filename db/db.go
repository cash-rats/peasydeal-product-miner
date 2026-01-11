package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"peasydeal-product-miner/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Conn interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	Queryx(query string, args ...any) (*sqlx.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowx(query string, args ...any) *sqlx.Row
	Prepare(query string) (*sql.Stmt, error)
	Preparex(query string) (*sqlx.Stmt, error)
	Rebind(query string) string
}

const (
	Production = "production"
	Preview    = "preview"
)

var ErrDBDisabled = errors.New("postgres disabled: set DB_* env vars to enable")

func GetPostgresqlDSN(cfg config.Config) string {
	pgdsn := "postgres://%s:%s@%s:%d/%s"
	params := ""

	if cfg.Env == Production || cfg.Env == Preview {
		params = "?sslmode=require"
	}

	return fmt.Sprintf(
		pgdsn,
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Name,
	) + params
}

type errConnector struct{}

func (errConnector) Connect(context.Context) (driver.Conn, error) { return nil, ErrDBDisabled }
func (errConnector) Driver() driver.Driver                        { return errDriver{} }

type errDriver struct{}

func (errDriver) Open(string) (driver.Conn, error) { return nil, ErrDBDisabled }

type disabledConn struct {
	db *sql.DB
	x  *sqlx.DB
}

func newDisabledConn() disabledConn {
	db := sql.OpenDB(errConnector{})
	return disabledConn{
		db: db,
		x:  sqlx.NewDb(db, "pgx"),
	}
}

func (c disabledConn) Exec(query string, args ...any) (sql.Result, error) {
	return c.db.Exec(query, args...)
}
func (c disabledConn) Query(query string, args ...any) (*sql.Rows, error) {
	return c.db.Query(query, args...)
}
func (c disabledConn) Queryx(query string, args ...any) (*sqlx.Rows, error) {
	return c.x.Queryx(query, args...)
}
func (c disabledConn) QueryRow(query string, args ...any) *sql.Row {
	return c.db.QueryRow(query, args...)
}
func (c disabledConn) QueryRowx(query string, args ...any) *sqlx.Row {
	return c.x.QueryRowx(query, args...)
}
func (c disabledConn) Prepare(query string) (*sql.Stmt, error)   { return c.db.Prepare(query) }
func (c disabledConn) Preparex(query string) (*sqlx.Stmt, error) { return c.x.Preparex(query) }
func (c disabledConn) Rebind(query string) string                { return c.x.Rebind(query) }

type SQLXOut struct {
	fx.Out

	DB   *sqlx.DB
	Conn Conn
}

func NewSQLXPostgresDB(lc fx.Lifecycle, cfg config.Config, logger *zap.Logger) (SQLXOut, error) {
	if strings.TrimSpace(cfg.DB.Host) == "" || strings.TrimSpace(cfg.DB.Name) == "" {
		logger.Info("postgres_disabled")
		return SQLXOut{DB: nil, Conn: newDisabledConn()}, nil
	}

	dsn := GetPostgresqlDSN(cfg)
	driver, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return SQLXOut{}, err
	}

	driver.SetMaxOpenConns(10)
	driver.SetMaxIdleConns(10)
	driver.SetConnMaxLifetime(30 * time.Minute)
	driver.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			return driver.PingContext(pingCtx)
		},
		OnStop: func(ctx context.Context) error {
			return driver.Close()
		},
	})

	logger.Info("postgres_enabled")
	return SQLXOut{DB: driver, Conn: driver}, nil
}
