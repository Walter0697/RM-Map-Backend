# RM-Map-Backend

### Introduction
- RoMarker Map, an application to store location that we want to visit. Just a personal project for myself to have practical usage

### Technologies
- GoFiber
- Postgres
- OIDC (default) with local password fallback for development
- GraphQL
- Simple Web scrapping using GoQuery

### Environment
- Follow `config.example.toml`.
- `app.authmode` controls auth behavior:
  - `oidc` for OpenID Connect login (recommended)
  - `local-password` for development/local testing only
- `REDIS_DB` can override `[redis].db` at runtime (expects a non-negative integer).
- `AUTH_SESSION_LIFETIME_SECONDS` can override `[app].authsessionttlseconds`.
- `AUTH_STATE_MIGRATION_MODE` can override `[authstate].migrationmode`.
- `AUTH_STATE_SESSION_TTL_SECONDS` can override `[authstate].sessionttlseconds`.
- `GET /auth/mode` shows active auth mode.
- `GET /auth/health` provides auth diagnostics.
- Optional OIDC/provider lifetime mirrors for alignment checks:
  - `[oidc].sessionttlseconds`
  - `[oidc].accesstokenttlseconds`
  - `[oidc].refreshtokenttlseconds`

### Authentication Token State (Postgres vs Redis)
- Login/session token state is controlled by `[authstate]` + `[redis]`.
- `migrationmode` options:
  - `dual-write`: Postgres-compatible mode. Validation is Postgres-first. Redis writes/backfills happen only when Redis is enabled.
  - `redis-primary`: Redis-first validation, Postgres fallback on Redis miss/error, then Redis backfill.
  - `postgres-off`: Redis-only validation/revocation (no Postgres fallback).

#### 1) Postgres-Compatible Setup (no Redis requirement)
Use this when you want token validation to keep working from Postgres:
```toml
[redis]
enable=false

[authstate]
migrationmode="dual-write"
keyprefix="auth:v1"
sessionttlseconds=31536000
```

#### 2) Redis Migration Setup (safe cutover path)
Use this when introducing Redis without hard cutover:
```toml
[redis]
enable=true
host="localhost"
port="6379"
password=""
db=0

[authstate]
migrationmode="redis-primary"
keyprefix="auth:v1"
sessionttlseconds=31536000
redisdialtimeoutms=1500
redisreadtimeoutms=1500
rediswritetimeoutms=1500
```

#### 3) Redis-Only Setup (after stabilization)
Use this only after verifying Redis reliability and fallback metrics:
```toml
[redis]
enable=true

[authstate]
migrationmode="postgres-off"
```

#### Runtime Overrides
- `REDIS_DB`: overrides `[redis].db`
- `AUTH_SESSION_LIFETIME_SECONDS`: overrides `[app].authsessionttlseconds`
- `AUTH_STATE_MIGRATION_MODE`: overrides `[authstate].migrationmode`
- `AUTH_STATE_SESSION_TTL_SECONDS`: overrides `[authstate].sessionttlseconds`

Example:
```bash
AUTH_SESSION_LIFETIME_SECONDS=31536000 AUTH_STATE_MIGRATION_MODE=redis-primary AUTH_STATE_SESSION_TTL_SECONDS=31536000 go run .
```

#### Recommended Rollout
1. Start with `dual-write` (Postgres-compatible baseline).
2. Move to `redis-primary` and watch `/auth/health` auth-state metrics (`validateFallbackCount`, `validateErrorCount`).
3. Move to `postgres-off` only after fallback/error metrics are stable.
4. Roll back quickly by setting `AUTH_STATE_MIGRATION_MODE=dual-write`.

### Auth Docs
- Migration checklist: `docs/auth-migration-checklist.md`
- Token state runbook: `docs/auth-token-state-runbook.md`
- Local Authentik guide: `docs/local-authentik-oidc.md`
- API key integration guide: `docs/api-key-integration.md`
- API key rollout checklist: `docs/api-key-rollout-checklist.md`
- Train station admin workflow: `docs/train-station-admin-workflow.md`

### Notes to self
run `go run -mod=mod github.com/99designs/gqlgen generate` if schema changed

### GitHub Actions CI and Delivery
- PRs run backend CI (`go test ./...`) and do not require repository secrets.
- Delivery runs only when the pushed branch equals the repository default branch.
- Backend image is published to GHCR as `ghcr.io/<owner>/rm-map-backend` with three tags:
  - commit SHA (first 12 chars)
  - version extracted from `server.go` (`current_version`)
  - `latest`
- Required repository permissions for delivery workflow job:
  - `contents: read`
  - `packages: write`
