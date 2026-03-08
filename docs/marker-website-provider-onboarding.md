# Marker Website Provider Onboarding

This document covers onboarding steps for marker website integrations (OpenRice, Yelp, Tabelog, and future providers).

## 1. Add provider constants

1. Add provider ID constant in `constant/scrapper.go`.
2. If outbound API calls are audited, add provider metadata in `service/external_api_provider_registry.go`.

## 2. Implement provider contract

1. Implement `MarkerWebsiteProvider` in `service/marker_website_provider.go`:
   - `ID()`
   - `ValidateExternalID(externalID string) error`
   - `FetchRestaurant(externalID string) (*dbmodel.Restaurant, error)`
2. Register provider in `markerWebsiteProviders`.

## 3. Wire API consumers

1. GraphQL mutation `websiteScrap` uses `GetOrCreateRestaurantByProvider`.
2. Integration API create/update marker handlers accept:
   - `website_provider`
   - `website_provider_id`
3. Ensure response includes `website_integration` normalized payload.

## 4. Validation and compatibility

1. Keep `restaurant_id` path backward-compatible.
2. Reject payloads that mix `restaurant_id` and provider fields.
3. Keep legacy OpenRice records compatible by mapping empty source + source_id to provider `openrice`.

## 5. Verification

1. Run backend tests:
   - `GOCACHE=/tmp/go-build go test ./service ./graph/...`
2. Run smoke script:
   - `scripts/smoke_marker_website_providers.sh`
