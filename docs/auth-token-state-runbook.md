# Auth Token State Runbook (Postgres and Redis)

This runbook describes how login/session token validation is configured and how to roll between Postgres-compatible mode and Redis-backed modes.

## Config Keys

- `[redis].enable`: turns Redis auth-state usage on/off.
- `[redis].host`, `[redis].port`, `[redis].password`, `[redis].db`: Redis connection details.
- `[authstate].migrationmode`: one of `dual-write`, `redis-primary`, `postgres-off`.
- `[authstate].keyprefix`: Redis key prefix for auth state (example: `auth:v1`).
- `[authstate].sessionttlseconds`: TTL for Redis auth session records.
- `[app].authsessionttlseconds`: canonical session lifetime policy (target one year).
- `[authstate].redisdialtimeoutms`, `[authstate].redisreadtimeoutms`, `[authstate].rediswritetimeoutms`: Redis timeouts.

Runtime overrides:
- `REDIS_DB`
- `AUTH_STATE_MIGRATION_MODE`
- `AUTH_STATE_SESSION_TTL_SECONDS`
- `AUTH_SESSION_LIFETIME_SECONDS`

## Mode Behavior

### `dual-write` (Postgres-compatible baseline)
- Writes token state to Postgres.
- If Redis is enabled, also writes to Redis.
- Validation is Postgres-first.
- Safe baseline for environments where Redis is optional.

### `redis-primary` (migration mode)
- Validation is Redis-first.
- On Redis miss/error, falls back to Postgres validation.
- Successful fallback backfills Redis.
- Recommended intermediate step before Redis-only mode.

### `postgres-off` (Redis-only)
- Validation/revocation rely on Redis only.
- If Redis is unavailable, auth validation returns `auth state unavailable`.
- Use only after Redis reliability is confirmed.

## Reference Configs

### Postgres-compatible baseline
```toml
[redis]
enable=false

[app]
authsessionttlseconds=31536000

[authstate]
migrationmode="dual-write"
keyprefix="auth:v1"
sessionttlseconds=31536000
```

### Redis migration
```toml
[redis]
enable=true
host="localhost"
port="6379"
password=""
db=0

[app]
authsessionttlseconds=31536000

[authstate]
migrationmode="redis-primary"
keyprefix="auth:v1"
sessionttlseconds=31536000
redisdialtimeoutms=1500
redisreadtimeoutms=1500
rediswritetimeoutms=1500
```

### Redis-only
```toml
[redis]
enable=true

[authstate]
migrationmode="postgres-off"
```

## Rollout Procedure

1. Start in `dual-write` and confirm login/logout/authorized API traffic works.
2. Enable Redis (`[redis].enable=true`) and switch to `redis-primary`.
3. Keep `[app].authsessionttlseconds` and `[authstate].sessionttlseconds` aligned.
4. Monitor `/auth/health`:
- `authState.metrics.validateFallbackCount`
- `authState.metrics.validateErrorCount`
- `authState.metrics.revocationErrorCount`
5. If metrics remain stable, switch to `postgres-off`.

## Rollback Procedure

1. Set `AUTH_STATE_MIGRATION_MODE=dual-write`.
2. Restart backend.
3. Verify `/auth/health` shows `authState.migrationMode = dual-write`.
4. Re-test login/logout and protected route access.

## Validation Checklist

- `GET /auth/mode` reports expected login mode (`oidc` or `local-password`).
- `GET /auth/health` reports expected `authState` block.
- Login creates a usable session.
- Logout invalidates session.
- Protected route access behaves correctly with valid/invalid token.
