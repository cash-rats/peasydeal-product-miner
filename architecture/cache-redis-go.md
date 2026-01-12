# `cache/redis.go` guide (normative)

This document defines the **exact** `cache/redis.go` implementation that must be used when initializing/adopting the VPS Go + FX + chi template in this repo.

Normative terms:
- **MUST / MUST NOT**: non-negotiable requirements.
- **SHOULD / SHOULD NOT**: strong defaults.
- **MAY**: optional.

## Hard rules

- Agents **MUST** implement `cache/redis.go` to match this document **verbatim** (public API, function signatures, behavior).
- Agents **MUST NOT** introduce alternate Redis packages, alternate constructors, or different gating semantics.
- Redis must be FX-wired and closed using `fx.Lifecycle` hooks.
- Server startup **MUST NOT** be blocked when Redis is disabled.

## Public types and functions (exact)

### Params struct

`NewRedisCacheParams` **MUST** exist as:

```go
type NewRedisCacheParams struct {
  fx.In

  Lifecycle fx.Lifecycle
  Config    *config.Config
  Logger    *zap.SugaredLogger
}
```

### Constructor

`NewRedis(p NewRedisCacheParams) (*redis.Client, error)` **MUST**:

- When disabled (`strings.TrimSpace(p.Config.Redis.Host) == ""`):
  - Log `p.Logger.Info("redis_disabled")`
  - Return `(nil, nil)`
- When enabled:
  - Build `redis.Options` (do not parse URLs):
    - `Addr: fmt.Sprintf("%s:%d", p.Config.Redis.Host, p.Config.Redis.Port)`
    - `Username: strings.TrimSpace(p.Config.Redis.User)`
    - `Password: strings.TrimSpace(p.Config.Redis.Password)`
    - `DB: int(p.Config.Redis.DB)`
  - Apply tuning values:
    - `PoolSize = 10`
    - `MinIdleConns = 1`
    - `MaxIdleConns = 5`
    - `ConnMaxIdleTime = 5 * time.Minute`
    - `ConnMaxLifetime = 30 * time.Minute`
    - `DialTimeout = 5 * time.Second`
    - `ReadTimeout = 3 * time.Second`
    - `WriteTimeout = 3 * time.Second`
    - `MaxRetries = 3`
    - `MinRetryBackoff = 8 * time.Millisecond`
    - `MaxRetryBackoff = 512 * time.Millisecond`
  - Create `client := redis.NewClient(opt)`
  - Register lifecycle hooks:
    - `OnStart`: `client.Ping` with a `2s` timeout
    - `OnStop`: `client.Close`
  - Log `p.Logger.Infow("redis_cache_initialized", "host", ..., "port", ..., "user", ...)`
  - Return `(client, nil)`

