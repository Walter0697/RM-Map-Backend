package service

import (
	"fmt"
	"testing"
	"time"

	"mapmarker/backend/config"
)

func TestGetPlanningWeatherSuccessAndCacheHit(t *testing.T) {
	originalConfig := config.Data
	originalFetch := weatherFetchProviderFn
	originalNow := weatherNowFn
	originalAudit := createExternalAPIAuditEventFn
	defer func() {
		config.Data = originalConfig
		weatherFetchProviderFn = originalFetch
		weatherNowFn = originalNow
		createExternalAPIAuditEventFn = originalAudit
	}()
	createExternalAPIAuditEventFn = func(input ExternalAPIAuditWriteInput) error { return nil }

	config.Data.Weather.Enable = true
	config.Data.Weather.Provider = ExternalAPIProviderOpenMeteo
	config.Data.Weather.CacheTTLSeconds = 600
	config.Data.Weather.StaleTTLSeconds = 3600
	config.Data.Weather.MaxForecastHours = 24
	config.Data.Weather.MaxViewportSpan = 30
	config.Data.Weather.MaxViewportPointStep = 1
	config.Data.Weather.RateLimitPerMinute = 120

	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	weatherNowFn = func() time.Time { return now }

	callCount := 0
	weatherFetchProviderFn = func(input WeatherPlanningInput) (weatherProviderResult, error) {
		callCount++
		return weatherProviderResult{
			ProviderTimestamp: now,
			Points: []weatherProviderPoint{
				{
					Lat:               input.CenterLat,
					Lon:               input.CenterLon,
					ForecastTimestamp: now.Add(1 * time.Hour),
					PrecipitationMM:   3.2,
					RainMM:            3.2,
					SnowMM:            0,
					ProviderTimestamp: now,
				},
			},
		}, nil
	}

	weatherCacheMu.Lock()
	weatherCacheData = make(map[string]weatherCacheEntry)
	weatherCacheMu.Unlock()

	input := WeatherPlanningInput{
		MinLat: -1, MaxLat: 1, MinLon: -1, MaxLon: 1,
		CenterLat: 0, CenterLon: 0, Zoom: 10, ForecastWindowH: 12,
	}

	first := GetPlanningWeather(input)
	second := GetPlanningWeather(input)

	if callCount != 1 {
		t.Fatalf("expected provider call once with cache hit, got %d", callCount)
	}
	if len(first.Items) == 0 || first.Items[0].PrecipitationType != "rain" {
		t.Fatalf("expected rain point, got %+v", first.Items)
	}
	if second.Freshness.Source != weatherSourceCache {
		t.Fatalf("expected cached response source, got %s", second.Freshness.Source)
	}
	if !second.Freshness.CacheHit {
		t.Fatalf("expected cache hit=true")
	}
}

func TestGetPlanningWeatherDegradedWithStaleCache(t *testing.T) {
	originalConfig := config.Data
	originalFetch := weatherFetchProviderFn
	originalNow := weatherNowFn
	originalAudit := createExternalAPIAuditEventFn
	defer func() {
		config.Data = originalConfig
		weatherFetchProviderFn = originalFetch
		weatherNowFn = originalNow
		createExternalAPIAuditEventFn = originalAudit
	}()
	createExternalAPIAuditEventFn = func(input ExternalAPIAuditWriteInput) error { return nil }

	config.Data.Weather.Enable = true
	config.Data.Weather.Provider = ExternalAPIProviderOpenMeteo
	config.Data.Weather.CacheTTLSeconds = 1
	config.Data.Weather.StaleTTLSeconds = 3600
	config.Data.Weather.MaxForecastHours = 24
	config.Data.Weather.MaxViewportSpan = 30
	config.Data.Weather.MaxViewportPointStep = 1

	start := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	now := start
	weatherNowFn = func() time.Time { return now }

	weatherFetchProviderFn = func(input WeatherPlanningInput) (weatherProviderResult, error) {
		if now.After(start.Add(2 * time.Second)) {
			return weatherProviderResult{}, fmt.Errorf("provider timeout")
		}
		return weatherProviderResult{
			ProviderTimestamp: now,
			Points: []weatherProviderPoint{
				{
					Lat:               0,
					Lon:               0,
					ForecastTimestamp: now.Add(1 * time.Hour),
					PrecipitationMM:   1,
					RainMM:            1,
					ProviderTimestamp: now,
				},
			},
		}, nil
	}

	weatherCacheMu.Lock()
	weatherCacheData = make(map[string]weatherCacheEntry)
	weatherCacheMu.Unlock()

	input := WeatherPlanningInput{
		MinLat: -1, MaxLat: 1, MinLon: -1, MaxLon: 1,
		CenterLat: 0, CenterLon: 0, Zoom: 10, ForecastWindowH: 12,
	}
	_ = GetPlanningWeather(input)
	now = now.Add(3 * time.Second)
	response := GetPlanningWeather(input)
	if response.Freshness.Status != weatherFreshnessStale {
		t.Fatalf("expected stale response, got %s", response.Freshness.Status)
	}
	if !response.Freshness.Degraded {
		t.Fatalf("expected degraded=true")
	}
}

func TestGetPlanningWeatherProviderFailureNoCache(t *testing.T) {
	originalConfig := config.Data
	originalFetch := weatherFetchProviderFn
	originalAudit := createExternalAPIAuditEventFn
	defer func() {
		config.Data = originalConfig
		weatherFetchProviderFn = originalFetch
		createExternalAPIAuditEventFn = originalAudit
	}()
	createExternalAPIAuditEventFn = func(input ExternalAPIAuditWriteInput) error { return nil }

	config.Data.Weather.Enable = true
	config.Data.Weather.Provider = ExternalAPIProviderOpenMeteo
	config.Data.Weather.MaxForecastHours = 24
	config.Data.Weather.MaxViewportSpan = 30
	config.Data.Weather.MaxViewportPointStep = 1

	weatherFetchProviderFn = func(input WeatherPlanningInput) (weatherProviderResult, error) {
		return weatherProviderResult{}, fmt.Errorf("timeout")
	}

	weatherCacheMu.Lock()
	weatherCacheData = make(map[string]weatherCacheEntry)
	weatherCacheMu.Unlock()

	input := WeatherPlanningInput{
		MinLat: -1, MaxLat: 1, MinLon: -1, MaxLon: 1,
		CenterLat: 0, CenterLon: 0, Zoom: 10, ForecastWindowH: 12,
	}
	response := GetPlanningWeather(input)
	if response.Freshness.Status != weatherFreshnessUnavailable {
		t.Fatalf("expected unavailable status, got %s", response.Freshness.Status)
	}
	if response.ErrorCode == "" {
		t.Fatalf("expected error code")
	}
}
