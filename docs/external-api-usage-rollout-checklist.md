# External API Usage Rollout / Rollback Checklist

## Rollout

1. Deploy backend with `external_api_audit_events` auto-migration enabled.
2. Confirm cleanup settings in `config.toml`:
   - `integrationauth.enablecleanup=true`
   - `integrationauth.logretentiondays` set to retention target
   - `integrationauth.maxauditlogrows` set for storage ceiling
3. Verify new endpoints return data for admin tokens:
   - `/admin/api-usage/providers`
   - `/admin/api-usage/summary`
   - `/admin/api-usage/trends`
4. Trigger TomTom and Movie DB calls and verify rows are written in `external_api_audit_events`.
5. Open frontend `/admin/api-usage` and verify summary cards, trend view, and provider breakdown render.
6. Monitor database write rate and query latency for:
   - insert throughput on `external_api_audit_events`
   - summary/trend response times for last 7 days
7. Validate cleanup worker removes old rows according to retention settings.

## Rollback

1. Hide frontend route by removing `/admin/api-usage` route/nav entries.
2. Disable writes by bypassing provider instrumentation wrapper calls.
3. Keep existing provider functionality active by falling back to direct outbound requests.
4. Leave historical `external_api_audit_events` rows in place unless storage pressure requires cleanup.
5. Re-run smoke tests for existing admin routes and provider integrations.

