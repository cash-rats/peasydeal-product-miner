# `config/config.go` guide (normative)

This document defines the **exact** `config/config.go` structure that must be used when initializing/adopting the VPS Go + FX + chi template in this repo.

Normative terms:
- **MUST / MUST NOT**: non-negotiable requirements.
- **SHOULD / SHOULD NOT**: strong defaults.
- **MAY**: optional.

## Hard rules

- Agents **MUST** implement `config/config.go` to match this document **verbatim** (types, field names, `mapstructure` tags, and viper defaults).
- Agents **MUST NOT** rename keys, introduce alternate env var names, or add extra defaults not listed here.
- Agents **MUST NOT** add unrelated config fields “for future use”.
- Config wiring **MUST** remain FX-provided via `config.NewViper` and `config.NewConfig`.
- Zero required env vars: defaults must allow `cmd/server` to start without Postgres/Redis/Inngest configured.

## ENV type

`config` package must define:

- `type ENV string`
- Vercel env values (exact):
  - `Production ENV = "production"`
  - `Preview ENV = "preview"`
  - `Dev ENV = "development"`
  - `Test ENV = "test"`

## `Config` struct (minimal)

`Config` is intentionally minimal for this project; it **MUST** be:

```go
type Config struct {
  ENV ENV `mapstructure:"vercel_env"`

  App struct {
    Port string `mapstructure:"port"`
    Addr string `mapstructure:"addr"`
  } `mapstructure:"app"`

  DB struct {
    Host     string `mapstructure:"host"`
    Port     uint   `mapstructure:"port"`
    User     string `mapstructure:"user"`
    Password string `mapstructure:"password"`
    Name     string `mapstructure:"name"`
    TimeZone string `mapstructure:"timezone"`
  } `mapstructure:"db"`

  Redis struct {
    User     string `mapstructure:"user"`
    Host     string `mapstructure:"host"`
    Port     uint   `mapstructure:"port"`
    Password string `mapstructure:"password"`
    DB       uint   `mapstructure:"db"`
  } `mapstructure:"redis"`

  Inngest struct {
    Dev        string `mapstructure:"dev"`
    AppID      string `mapstructure:"app_id"`
    SigningKey string `mapstructure:"signing_key"`
    ServeHost  string `mapstructure:"serve_host"`
    ServePath  string `mapstructure:"serve_path"`
  } `mapstructure:"inngest"`
}
```

## `NewViper()` (defaults + env mapping)

`NewViper()` **MUST**:

- Create a fresh `*viper.Viper`
- Set defaults exactly as listed below
- Set env key replacer: `strings.NewReplacer(".", "_")`
- Call `vp.AutomaticEnv()`

### Defaults (exact)

```go
vp.SetDefault("vercel_env", Dev)
vp.SetDefault("app.port", "8080")
vp.SetDefault("app.addr", "0.0.0.0")

vp.SetDefault("db.host", "")
vp.SetDefault("db.port", 5432)
vp.SetDefault("db.user", "")
vp.SetDefault("db.password", "")
vp.SetDefault("db.name", "")
vp.SetDefault("db.timezone", "")

vp.SetDefault("redis.host", "")
vp.SetDefault("redis.user", "default")
vp.SetDefault("redis.port", 6379)
vp.SetDefault("redis.password", "")
vp.SetDefault("redis.db", 0)

vp.SetDefault("inngest.dev", "")
vp.SetDefault("inngest.app_id", "")
vp.SetDefault("inngest.signing_key", "")
vp.SetDefault("inngest.serve_host", "")
vp.SetDefault("inngest.serve_path", "")
```

### Env var names (derived)

Because the replacer converts `.` to `_`, the following env vars are supported (non-exhaustive examples):

- `VERCEL_ENV`
- `APP_PORT`, `APP_ADDR`
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_TIMEZONE`
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_USER`, `REDIS_PASSWORD`, `REDIS_DB`
- `INNGEST_DEV`, `INNGEST_APP_ID`, `INNGEST_SIGNING_KEY`, `INNGEST_SERVE_HOST`, `INNGEST_SERVE_PATH`

## `NewConfig()` (unmarshal)

`NewConfig(vp *viper.Viper)` **MUST**:

- Unmarshal into `*Config` using `vp.Unmarshal`
- Return `(*Config, error)` (do not `log.Fatal` inside config)

