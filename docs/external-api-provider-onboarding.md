# External API Provider Onboarding

This project uses a shared outbound API instrumentation contract for provider usage analytics.

## Steps

1. Add provider metadata in `service/external_api_provider_registry.go`.
2. Choose normalized operation names for each outbound call (example: `search_movie`, `reverse_geocode`).
3. Route all provider HTTP calls through `GetRequestWithExternalAPIAudit(provider, operation, url, validateBody)`.
4. In `validateBody`, decode only what is required and return errors as `decode response: ...` to classify local decode failures.
5. Never persist request or response payloads in audit events. Use metadata fields only.
6. Verify the provider appears in admin API metadata:
   - `GET /admin/api-usage/providers`
7. Verify admin summary and trend endpoints with provider filter:
   - `GET /admin/api-usage/summary?provider=<id>`
   - `GET /admin/api-usage/trends?provider=<id>&interval=1h`
8. Confirm the frontend `/admin/api-usage` page renders the provider without provider-specific code changes.

