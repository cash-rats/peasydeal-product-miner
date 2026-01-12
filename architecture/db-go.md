# `db/db.go` guide (normative)

This document defines the **exact** `db/db.go` implementation that must be used when initializing/adopting the VPS Go + FX + chi template in this repo.

Normative terms:
- **MUST / MUST NOT**: non-negotiable requirements.
- **SHOULD / SHOULD NOT**: strong defaults.
- **MAY**: optional.

## Hard rules

- Agents **MUST** implement `db/db.go` to match this document **verbatim** (public API, function signatures, behavior).
- Agents **MUST NOT** introduce alternate DB packages, alternate constructors, or different gating semantics.
- DB must be FX-wired and closed using `fx.Lifecycle` hooks.
- Server startup **MUST NOT** be blocked when DB is disabled.

## Package + imports

- Package: `package db`
- Must use SQLX + pgx:
  - Blank import: `_ "github.com/jackc/pgx/v5/stdlib"`
  - `github.com/jmoiron/sqlx`
  - `github.com/jmoiron/sqlx/reflectx`
- Must accept config as `*config.Config` from `peasydeal-product-miner/config`.

## Public types and functions (exact)

### `Conn` interface

`Conn` **MUST** exist and include the following methods (exact names/signatures):

```go
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
```

### Disable semantics (required)

- `var ErrDBDisabled = errors.New("postgres disabled: set DB_* env vars to enable")` **MUST** exist.
- When DB is disabled, calls through `Conn` must return `ErrDBDisabled` (via a disabled driver/connector), not panic.
- DB is considered disabled when either of these is empty/whitespace:
  - `cfg.DB.Host`
  - `cfg.DB.Name`

### DSN builder

`GetPostgresqlDSN(cfg *config.Config) string` **MUST**:

- Build `postgres://%s:%s@%s:%d/%s`
- Append `?sslmode=require` only when:
  - `cfg.ENV == config.Production` OR `cfg.ENV == config.Preview`

### FX output type

`SQLXOut` **MUST** exist as:

```go
type SQLXOut struct {
  fx.Out

  DB   *sqlx.DB
  Conn Conn
}
```

### Constructor

`NewSQLXPostgresDB(lc fx.Lifecycle, cfg *config.Config, logger *zap.SugaredLogger) (SQLXOut, error)` **MUST**:

- When disabled:
  - `logger.Infow("postgres_disabled")`
  - Return `{DB: nil, Conn: <disabledConn>}`, `nil`
- When enabled:
  - Use `sqlx.Open("pgx", GetPostgresqlDSN(cfg))`
  - Set pool settings:
    - `SetMaxOpenConns(10)`
    - `SetMaxIdleConns(10)`
    - `SetConnMaxLifetime(30 * time.Minute)`
  - Set mapper: `reflectx.NewMapperFunc("json", strings.ToLower)`
  - Register lifecycle hooks:
    - `OnStart`: `PingContext` with a `5s` timeout
    - `OnStop`: `Close`
  - `logger.Infow("postgres_enabled")`
  - Return `{DB: driver, Conn: driver}`, `nil`

