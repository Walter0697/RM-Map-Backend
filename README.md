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
- `GET /auth/mode` shows active auth mode.
- `GET /auth/health` provides auth diagnostics.

### Auth Docs
- Migration checklist: `docs/auth-migration-checklist.md`
- Local Authentik guide: `docs/local-authentik-oidc.md`
- API key integration guide: `docs/api-key-integration.md`
- API key rollout checklist: `docs/api-key-rollout-checklist.md`

### Notes to self
run `go run -mod=mod github.com/99designs/gqlgen generate` if schema changed
