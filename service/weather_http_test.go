package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mapmarker/backend/config"
)

func TestPlanningWeatherHandlerValidation(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()
	config.Data.Weather.Enable = true
	config.Data.Weather.MaxForecastHours = 24
	config.Data.Weather.MaxViewportSpan = 30

	req := httptest.NewRequest(http.MethodGet, "/weather/planning?min_lat=bad", nil)
	rec := httptest.NewRecorder()
	PlanningWeatherHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPlanningWeatherHandlerFeatureDisabled(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()
	config.Data.Weather.Enable = false
	req := httptest.NewRequest(http.MethodGet, "/weather/planning", nil)
	rec := httptest.NewRecorder()
	PlanningWeatherHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestPlanningWeatherHandlerRainSnowScenario(t *testing.T) {
	originalConfig := config.Data
	originalFetch := weatherFetchProviderFn
	originalAudit := createExternalAPIAuditEventFn
	originalNow := weatherNowFn
	defer func() {
		config.Data = originalConfig
		weatherFetchProviderFn = originalFetch
		createExternalAPIAuditEventFn = originalAudit
		weatherNowFn = originalNow
	}()

	config.Data.Weather.Enable = true
	config.Data.Weather.Provider = ExternalAPIProviderOpenMeteo
	config.Data.Weather.CacheTTLSeconds = 600
	config.Data.Weather.StaleTTLSeconds = 3600
	config.Data.Weather.MaxForecastHours = 24
	config.Data.Weather.MaxViewportSpan = 30
	config.Data.Weather.MaxViewportPointStep = 1
	config.Data.Weather.RateLimitPerMinute = 120
	now := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	weatherNowFn = func() time.Time { return now }
	createExternalAPIAuditEventFn = func(input ExternalAPIAuditWriteInput) error { return nil }
	weatherFetchProviderFn = func(input WeatherPlanningInput) (weatherProviderResult, error) {
		return weatherProviderResult{
			ProviderTimestamp: now,
			TemperatureUnit:   "°C",
			Points: []weatherProviderPoint{
				{
					Lat:               22.30,
					Lon:               114.17,
					ForecastTimestamp: now.Add(1 * time.Hour),
					PrecipitationMM:   2.0,
					RainMM:            2.0,
					SnowMM:            0,
					ProviderTimestamp: now,
					Temperature:       ptrFloat(21.5),
				},
				{
					Lat:               22.31,
					Lon:               114.18,
					ForecastTimestamp: now.Add(1 * time.Hour),
					PrecipitationMM:   1.2,
					RainMM:            0,
					SnowMM:            1.2,
					ProviderTimestamp: now,
					Temperature:       ptrFloat(19.0),
				},
			},
		}, nil
	}
	weatherCacheMu.Lock()
	weatherCacheData = make(map[string]weatherCacheEntry)
	weatherCacheMu.Unlock()

	req := httptest.NewRequest(
		http.MethodGet,
		"/weather/planning?min_lat=22.29&max_lat=22.32&min_lon=114.16&max_lon=114.19&center_lat=22.305&center_lon=114.175&zoom=11&forecast_window_h=24",
		nil,
	)
	rec := httptest.NewRecorder()
	PlanningWeatherHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	payload := WeatherPlanningResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two overlay points, got %d", len(payload.Items))
	}
	if payload.Items[0].PrecipitationType != "rain" {
		t.Fatalf("expected first point to be rain, got %s", payload.Items[0].PrecipitationType)
	}
	if payload.Items[1].PrecipitationType != "snow" {
		t.Fatalf("expected second point to be snow, got %s", payload.Items[1].PrecipitationType)
	}
}
