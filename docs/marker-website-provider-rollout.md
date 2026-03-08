# Marker Website Provider Rollout / Rollback

## Rollout

1. Deploy backend with provider registry and integration API payload changes.
2. Deploy frontend with Yelp/Tabelog scraper + provider-agnostic restaurant rendering.
3. Run smoke checks:
   - `scripts/smoke_marker_website_providers.sh`
4. Monitor:
   - integration marker create error rates (`invalid_website_integration`)
   - external API audit events for `openrice`, `yelp`, and `tabelog`

## Rollback

1. Disable new provider usage from clients (stop sending `website_provider*` fields).
2. Revert frontend provider menu to OpenRice-only if needed.
3. Keep backend backward compatibility path active (`restaurant_id` unchanged).
4. If provider fetches degrade reliability:
   - keep marker core payloads (non-blocking)
   - stop provider fetch path by reverting provider registration entries in `markerWebsiteProviders`.
