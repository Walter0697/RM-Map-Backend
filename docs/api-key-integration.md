# API Key Integration Guide

## Overview

This project supports machine-to-machine integration using API keys with explicit scopes.

Supported scopes:
- `markers:read`
- `markers:write`
- `schedules:read`
- `schedules:write`
- `stations:read`
- `stations:write`
- `settings:read`
- `settings:write`

## Management Endpoints

These endpoints require authenticated admin JWT user context:

- `GET /auth/apikeys`
- `GET /auth/apikeys/options`
- `POST /auth/apikeys`
- `POST /auth/apikeys/{id}/revoke`
- `POST /auth/apikeys/{id}/rotate`

Options endpoint response includes:
- `users`: available actor users (`id`, `username`, `role`)
- `relations`: available relation pairs (`id`, user pair info and `display`)

Create request example:

```json
{
  "name": "integration-bot",
  "scopes": ["markers:read", "markers:write"],
  "relation_id": 1,
  "actor_user_id": 1,
  "expires_at": "2026-12-31T23:59:59Z"
}
```

## Integration Endpoints

Provide API key in `X-API-Key` (or `Authorization: ApiKey <token>`):

- `GET /integration/markers`
- `POST /integration/markers`
- `PUT /integration/markers/{id}`
- `GET /integration/schedules?time=YYYY-MM-DD`
- `POST /integration/schedules`
- `GET /integration/stations`
- `PUT /integration/stations`
- `GET /integration/settings/pins`
- `GET /integration/settings/marker-types`
- `GET /integration/settings/default-pins`
- `PUT /integration/settings/default-pins/{label}`

JWT user auth is not required for these integration endpoints and should not be used for automation.

### List Query Controls

List endpoints support:

- `limit` (default `50`, max `200`)
- `offset` (default `0`)
- `sort_by`
- `order` (`asc` / `desc`)

Endpoint-specific filters:

- `GET /integration/markers`
  - `type`, `status`, `country`, `country_code`, `label`, `search`
- `GET /integration/schedules`
  - `time` (`YYYY-MM-DD`), `status`, `marker_id`, `label`, `search`, `from` (`RFC3339`), `to` (`RFC3339`)
- `GET /integration/stations`
  - `map_name`, `identifier`, `label`, `active`
- `GET /integration/settings/pins`
  - `label`
- `GET /integration/settings/marker-types`
  - `label`, `value`, `hidden`
- `GET /integration/settings/default-pins`
  - `label`

List responses include metadata:

```json
{
  "items": [],
  "total": 0,
  "limit": 50,
  "offset": 0,
  "sort_by": "label",
  "order": "asc"
}
```

## Audit Logging

Every integration request writes API key audit logs with:
- source IP
- key name/identifier
- operation
- outcome and failure reason

## Retention and Cleanup

Configure in `config.toml`:

```toml
[integrationauth]
enablecleanup=true
logretentiondays=30
maxauditlogrows=200000
revokedkeyretentiondays=30
cleanupintervalhours=24
```

Cleanup worker removes expired logs and stale revoked keys while keeping active keys intact.
