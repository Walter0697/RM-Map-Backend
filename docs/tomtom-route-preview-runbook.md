# TomTom Route Preview Runbook

## Purpose

This runbook covers runtime setup and failure handling for route planning and route static-image generation.

## Required Configuration

Set these values in `config.toml` (or env-injected equivalent):

- `[apikey].tomtomrouting`: TomTom routing API key
- `[apikey].tomtomstaticimage`: TomTom static image API key
- `[apikey].tomtommap`: fallback key used when specific routing/static keys are not set
- `[tomtomroute].baseurl`: default `https://api.tomtom.com/routing/1`
- `[tomtomroute].timeoutms`: routing request timeout in milliseconds
- `[tomtomroute].retrycount`: retries for transient 5xx/timeout route failures
- `[tomtomstaticimage].baseurl`: default `https://api.tomtom.com/map/1/staticimage`
- `[tomtomstaticimage].timeoutms`: static-image request timeout in milliseconds

## Endpoints

- `POST /integration/routes/plan`
- `POST /integration/routes/static-image`

Both endpoints require the API key scope `static-preview:generate`.

## Error and Degradation Behavior

- Invalid coordinates return `invalid_route_input` (HTTP 400).
- Upstream quota saturation returns `route_provider_rate_limited` (HTTP 429).
- Retriable dependency failures return `route_provider_retriable_failure` (HTTP 502).
- Non-retriable provider failures return `route_provider_failed` (HTTP 502).
- Route mode ETA failures are non-fatal: response still returns a route with per-mode availability flags and warnings.
- Schedule route preview attachment is non-fatal: schedule create/update still succeeds and warnings are returned.

## Telemetry

Provider calls are recorded through external API audit events with provider `tomtom_map`:

- Route planning operations: `route_plan_base`, `route_plan_walking`, `route_plan_bus`, `route_plan_public_transit`
- Static image operation: `route_static_image`

Use existing API usage dashboards (`/admin/api-usage`) to track:

- error-rate increases
- sustained 429 responses (quota pressure)
- latency regressions

## Rollback

If route preview causes instability:

1. Stop clients from calling `/integration/routes/*`.
2. Leave schedule route metadata fields in place (backward-compatible nullable fields).
3. Keep existing schedule flows active; route metadata can remain empty.
