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
- `static-preview:generate`

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
- `DELETE /integration/markers/{id}`
- `GET /integration/schedules?time=YYYY-MM-DD`
- `POST /integration/schedules`
- `GET /integration/stations`
- `PUT /integration/stations`
- `GET /integration/settings/pins`
- `GET /integration/settings/marker-types`
- `GET /integration/settings/default-pins`
- `PUT /integration/settings/default-pins/{label}`
- `GET /integration/settings/users/{username}/preview-pin`
- `PUT /integration/settings/users/{username}/preview-pin`
- `POST /integration/static-map-preview/geocode`
- `POST /integration/static-map-preview`

JWT user auth is not required for these integration endpoints and should not be used for automation.

### List Query Controls

List endpoints support:

- `limit` (default `50`, max `200`)
- `offset` (default `0`)
- `cursor` (optional, use instead of `offset` for cursor pagination)
- `sort_by`
- `order` (`asc` / `desc`)

Endpoint-specific filters:

- `GET /integration/markers`
  - `type`, `status`, `country`, `country_code`, `label`, `search`
  - `west`, `south`, `east`, `north` (viewport bounding box, all required together)
  - `zoom` (optional zoom hint)
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

### Static Preview Generation Contract

Geocoding request:

```json
{
  "street_number": "100",
  "street_name": "Nathan Road",
  "country": "Hong Kong"
}
```

Geocoding response:

```json
{
  "lat": 22.302711,
  "lon": 114.177216
}
```

Preview request:

```json
{
  "username": "alice",
  "lat": 22.302711,
  "lon": 114.177216
}
```

Preview response:

```json
{
  "username": "alice",
  "lat": 22.302711,
  "lon": 114.177216,
  "image_base64": "<base64-png>",
  "mime_type": "image/png",
  "format": "png",
  "width": 600,
  "height": 400
}
```

Error response format:

```json
{
  "code": "invalid_coordinates",
  "message": "lat and lon must be within valid ranges"
}
```

Common error codes for `POST /integration/static-map-preview/geocode`:
- `invalid_payload`
- `invalid_address_input`
- `geocode_dependency_failure`

Common error codes for `POST /integration/static-map-preview`:
- `invalid_payload`
- `invalid_username`
- `invalid_marker_type_input`
- `invalid_location_input`
- `invalid_coordinates`
- `unknown_username`
- `preview_pin_not_configured`
- `invalid_preview_pin`
- `tomtom_dependency_failure`
- `image_composition_failure`

Address-mode flow (street -> geocode -> preview):

1. Call `POST /integration/static-map-preview/geocode`.
2. Use returned `lat` and `lon` in `POST /integration/static-map-preview`.


### User Preview Pin Setup

1. Ensure API key includes `settings:write` and `settings:read`.
2. Inspect available pins via `GET /integration/settings/pins`.
3. Save user preview pin with `PUT /integration/settings/users/{username}/preview-pin`:

```json
{
  "pin_id": 12
}
```

4. Verify selection with `GET /integration/settings/users/{username}/preview-pin`.
5. Call `POST /integration/static-map-preview` using key with `static-preview:generate`.
   If you have street-style input, call `POST /integration/static-map-preview/geocode` first.

### Operational Troubleshooting

- `unknown_username`: username must exactly match an existing user record.
- `preview_pin_not_configured`: ensure a user-selected preview pin exists or configure the system default preview pin.
- `invalid_preview_pin`: selected pin no longer exists or is inactive.
- `tomtom_dependency_failure`: verify TomTom API key and outbound connectivity.
- `image_composition_failure`: verify pin image assets exist under `uploads/pins`.
- `invalid_marker_type_input`: ensure `marker_type_name` matches an existing marker type `value` or `label`.

### Non-Production Validation (n8n-style)

Example cURL flow:

```bash
curl -X PUT "$BASE_URL/integration/settings/users/alice/preview-pin" \
  -H "X-API-Key: $SETTINGS_KEY" \
  -H "Content-Type: application/json" \
  -d '{"pin_id":12}'

curl -X POST "$BASE_URL/integration/static-map-preview/geocode" \
  -H "X-API-Key: $PREVIEW_KEY" \
  -H "Content-Type: application/json" \
  -d '{"street_number":"100","street_name":"Nathan Road","country":"Hong Kong"}'

curl -X POST "$BASE_URL/integration/static-map-preview" \
  -H "X-API-Key: $PREVIEW_KEY" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","marker_type_name":"food","lat":22.302711,"lon":114.177216}'
```

List responses include metadata:

```json
{
  "items": [],
  "total": 0,
  "limit": 50,
  "offset": 0,
  "cursor": 0,
  "nextCursor": "",
  "sort_by": "label",
  "order": "asc"
}
```

### Marker Creation Indicators (Integration API)

Integration marker create/update/list responses include extra fields:
- `integration_source`: always `api_key`
- `created_ago`: humanized age from `created_at` (for example, `just now`, `3 hours ago`)
- `editable_with_same_api_key`: boolean
- `removable_with_same_api_key`: boolean

You still get the marker `id` in create response, so you can immediately update or remove it.

Delete marker example:

```bash
curl -X DELETE "$BASE_URL/integration/markers/123" \
  -H "X-API-Key: $MARKER_WRITE_KEY"
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
