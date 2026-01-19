package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"peasydeal-product-miner/config"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"go.uber.org/fx"
	"go.uber.org/zap"

	// Turso "remote only" driver (no embedded replicas)
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

var ErrSQLiteDisabled = errors.New("turso sqlite disabled: set TURSO_DATABASE_URL (and TURSO_AUTH_TOKEN)")

// --- disabled connection (keeps app booting, but fails fast when used) ---

type sqliteErrConnector struct{}

func (sqliteErrConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, ErrSQLiteDisabled
}
func (sqliteErrConnector) Driver() driver.Driver { return sqliteErrDriver{} }

type sqliteErrDriver struct{}

func (sqliteErrDriver) Open(string) (driver.Conn, error) { return nil, ErrSQLiteDisabled }

type disabledSQLiteConn struct {
	db *sql.DB
	x  *sqlx.DB
}

func newDisabledSQLiteConn() disabledSQLiteConn {
	db := sql.OpenDB(sqliteErrConnector{})
	return disabledSQLiteConn{
		db: db,
		x:  sqlx.NewDb(db, "libsql"),
	}
}

// If your Conn interface needs more methods, add them here.
// (Keeping yours minimal; call sites will see ErrSQLiteDisabled quickly.)
func (c disabledSQLiteConn) Exec(query string, args ...any) (sql.Result, error) {
	return nil, ErrSQLiteDisabled
}
func (c disabledSQLiteConn) Query(query string, args ...any) (*sql.Rows, error) {
	return nil, ErrSQLiteDisabled
}
func (c disabledSQLiteConn) Queryx(query string, args ...any) (*sqlx.Rows, error) {
	return nil, ErrSQLiteDisabled
}
func (c disabledSQLiteConn) QueryRow(query string, args ...any) *sql.Row {
	return c.db.QueryRow(query, args...)
}
func (c disabledSQLiteConn) QueryRowx(query string, args ...any) *sqlx.Row {
	return c.x.QueryRowx(query, args...)
}
func (c disabledSQLiteConn) Prepare(query string) (*sql.Stmt, error)   { return nil, ErrSQLiteDisabled }
func (c disabledSQLiteConn) Preparex(query string) (*sqlx.Stmt, error) { return nil, ErrSQLiteDisabled }
func (c disabledSQLiteConn) Rebind(query string) string                { return c.x.Rebind(query) }

// --- Fx output ---

type SQLiteSQLXOut struct {
	fx.Out

	DB   *sqlx.DB `name:"sqlite"`
	Conn Conn     `name:"sqlite"`
}

type NewSQLXSQLiteDBParams struct {
	fx.In

	Lc     fx.Lifecycle
	Cfg    *config.Config
	Logger *zap.SugaredLogger
}

// NewSQLXSQLiteDB connects to Turso remote (libsql://...) using libsql-client-go.
func NewSQLXSQLiteDB(p NewSQLXSQLiteDBParams) (SQLiteSQLXOut, error) {
	// Prefer DSN if you already set it; otherwise use DatabaseURL/Path field naming you have.
	// For Turso remote, this should be: libsql://<db>.turso.io
	dsn := strings.TrimSpace(p.Cfg.Turso.DSN)
	if dsn == "" {
		dsn = strings.TrimSpace(p.Cfg.Turso.Path)
	}

	if dsn == "" {
		p.Logger.Infow("turso_sqlite_disabled")
		return SQLiteSQLXOut{DB: nil, Conn: newDisabledSQLiteConn()}, nil
	}

	dsn = ensureAuthTokenQuery(dsn, p.Cfg.Turso.Token)

	// Turso quickstart uses driver name "libsql" for remote-only.  [oai_citation:3‡docs.turso.tech](https://docs.turso.tech/sdk/go/quickstart)
	db, err := sqlx.Open("libsql", dsn)
	if err != nil {
		return SQLiteSQLXOut{}, fmt.Errorf("open turso db: %w", err)
	}

	// Reasonable defaults for remote DB:
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)

	p.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := db.PingContext(pingCtx); err != nil {
				_ = db.Close()
				return fmt.Errorf("ping turso db: %w", err)
			}
			p.Logger.Infow("turso_sqlite_enabled", "driver", "libsql")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	// Assuming *sqlx.DB satisfies your Conn interface (your original code returned it as Conn).
	return SQLiteSQLXOut{DB: db, Conn: db}, nil
}

func ensureAuthTokenQuery(dsn, token string) string {
	if token == "" {
		return dsn
	}

	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return dsn
	}

	// Don’t add tokens to local sqlite/file DSNs.
	if strings.EqualFold(u.Scheme, "file") || strings.EqualFold(u.Scheme, "sqlite") {
		return dsn
	}

	q := u.Query()
	if q.Get("authToken") != "" {
		return dsn
	}

	// Turso Go quickstart uses authToken query param.  [oai_citation:4‡docs.turso.tech](https://docs.turso.tech/sdk/go/quickstart)
	q.Set("authToken", token)
	u.RawQuery = q.Encode()
	return u.String()
}
