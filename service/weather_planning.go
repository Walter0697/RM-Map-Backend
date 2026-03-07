package service

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
)

const (
	weatherFreshnessFresh       = "fresh"
	weatherFreshnessStale       = "stale"
	weatherFreshnessUnavailable = "unavailable"
	weatherSourceProvider       = "provider"
	weatherSourceCache          = "cache"
	weatherSourceDegraded       = "degraded"
	weatherErrorProviderFailure = "provider_failure"
	weatherErrorProviderQuota   = "provider_quota_exceeded"
	weatherErrorProviderTimeout = "provider_timeout"
)

type WeatherPlanningInput struct {
	MinLat            float64
	MaxLat            float64
	MinLon            float64
	MaxLon            float64
	CenterLat         float64
	CenterLon         float64
	Zoom              float64
	ForecastWindowH   int
	ForecastDayOffset int
}

type WeatherPlanningResponse struct {
	Viewport     WeatherViewportBounds `json:"viewport"`
	Items        []WeatherOverlayPoint `json:"items"`
	Freshness    WeatherFreshnessMeta  `json:"freshness"`
	Provider     string                `json:"provider"`
	GeneratedAt  time.Time             `json:"generated_at"`
	ErrorCode    string                `json:"error_code,omitempty"`
	ErrorMessage string                `json:"error_message,omitempty"`
}

type WeatherViewportBounds struct {
	MinLat float64 `json:"min_lat"`
	MaxLat float64 `json:"max_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLon float64 `json:"max_lon"`
	Zoom   float64 `json:"zoom"`
}

type WeatherOverlayPoint struct {
	Lat               float64   `json:"lat"`
	Lon               float64   `json:"lon"`
	PrecipitationType string    `json:"precipitation_type"`
	Intensity         string    `json:"intensity"`
	PrecipitationMM   float64   `json:"precipitation_mm"`
	RainMM            float64   `json:"rain_mm"`
	SnowMM            float64   `json:"snow_mm"`
	ForecastTimestamp time.Time `json:"forecast_timestamp"`
	ProviderTimestamp time.Time `json:"provider_timestamp"`
	RetrievedAt       time.Time `json:"retrieved_at"`
	Freshness         string    `json:"freshness"`
}

type WeatherFreshnessMeta struct {
	Status            string    `json:"status"`
	ProviderTimestamp time.Time `json:"provider_timestamp,omitempty"`
	RetrievedAt       time.Time `json:"retrieved_at,omitempty"`
	AgeSeconds        int64     `json:"age_seconds"`
	Stale             bool      `json:"stale"`
	Degraded          bool      `json:"degraded"`
	Source            string    `json:"source"`
	CacheHit          bool      `json:"cache_hit"`
}

type weatherProviderPoint struct {
	Lat               float64
	Lon               float64
	ForecastTimestamp time.Time
	PrecipitationMM   float64
	RainMM            float64
	SnowMM            float64
	ProviderTimestamp time.Time
}

type weatherProviderResult struct {
	Points            []weatherProviderPoint
	ProviderTimestamp time.Time
}

type weatherCacheEntry struct {
	Response    WeatherPlanningResponse
	StoredAtUTC time.Time
}

type weatherRateLimiter struct {
	mu           sync.Mutex
	windowStart  time.Time
	requestCount int
}

func (l *weatherRateLimiter) allow(now time.Time, limit int) bool {
	if limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windowStart.IsZero() || now.Sub(l.windowStart) >= time.Minute {
		l.windowStart = now
		l.requestCount = 0
	}
	if l.requestCount >= limit {
		return false
	}
	l.requestCount++
	return true
}

var (
	weatherCacheMu   sync.RWMutex
	weatherCacheData = make(map[string]weatherCacheEntry)
	weatherLimiter   = &weatherRateLimiter{}
)

var weatherFetchProviderFn = fetchOpenMeteoPlanningForecast
var weatherNowFn = func() time.Time { return time.Now().UTC() }

func WeatherPlanningEnabled() bool {
	return config.Data.Weather.Enable
}

func GetPlanningWeather(input WeatherPlanningInput) WeatherPlanningResponse {
	now := weatherNowFn()
	normalizedInput, validationErr := normalizeWeatherInput(input)
	if validationErr != nil {
		return WeatherPlanningResponse{
			Viewport: WeatherViewportBounds{
				MinLat: input.MinLat,
				MaxLat: input.MaxLat,
				MinLon: input.MinLon,
				MaxLon: input.MaxLon,
				Zoom:   input.Zoom,
			},
			Items:        []WeatherOverlayPoint{},
			Freshness:    WeatherFreshnessMeta{Status: weatherFreshnessUnavailable, Stale: true, Degraded: true, Source: weatherSourceDegraded},
			Provider:     strings.TrimSpace(config.Data.Weather.Provider),
			GeneratedAt:  now,
			ErrorCode:    "invalid_input",
			ErrorMessage: "invalid weather viewport input",
		}
	}

	cacheKey := weatherCacheKey(normalizedInput)
	if entry, ok := weatherCacheGet(cacheKey); ok {
		age := now.Sub(entry.StoredAtUTC)
		if age <= weatherCacheTTL() {
			response := entry.Response
			response.GeneratedAt = now
			response.Freshness.CacheHit = true
			response.Freshness.Source = weatherSourceCache
			response.Freshness.AgeSeconds = int64(age.Seconds())
			weatherRecordCacheAudit("planning_forecast_cache_hit", dbmodel.ExternalAPIAuditStatusSuccess, now)
			return response
		}
	}
	weatherRecordCacheAudit("planning_forecast_cache_miss", dbmodel.ExternalAPIAuditStatusSuccess, now)

	if !weatherLimiter.allow(now, config.Data.Weather.RateLimitPerMinute) {
		return weatherDegradedFromCacheOrUnavailable(cacheKey, normalizedInput, now, weatherErrorProviderQuota, "weather provider quota exceeded")
	}

	result, fetchErr := weatherFetchProviderFn(normalizedInput)
	if fetchErr != nil {
		errorCode := weatherErrorProviderFailure
		if strings.Contains(strings.ToLower(fetchErr.Error()), "timeout") {
			errorCode = weatherErrorProviderTimeout
		}
		return weatherDegradedFromCacheOrUnavailable(cacheKey, normalizedInput, now, errorCode, fetchErr.Error())
	}

	response := WeatherPlanningResponse{
		Viewport: WeatherViewportBounds{
			MinLat: normalizedInput.MinLat,
			MaxLat: normalizedInput.MaxLat,
			MinLon: normalizedInput.MinLon,
			MaxLon: normalizedInput.MaxLon,
			Zoom:   normalizedInput.Zoom,
		},
		Items:       make([]WeatherOverlayPoint, 0, len(result.Points)),
		Provider:    strings.TrimSpace(config.Data.Weather.Provider),
		GeneratedAt: now,
		Freshness: WeatherFreshnessMeta{
			Status:            weatherFreshnessFresh,
			ProviderTimestamp: result.ProviderTimestamp,
			RetrievedAt:       now,
			AgeSeconds:        0,
			Stale:             false,
			Degraded:          false,
			Source:            weatherSourceProvider,
			CacheHit:          false,
		},
	}

	for _, item := range result.Points {
		precipType, intensity := classifyPrecipitation(item.RainMM, item.SnowMM, item.PrecipitationMM)
		response.Items = append(response.Items, WeatherOverlayPoint{
			Lat:               item.Lat,
			Lon:               item.Lon,
			PrecipitationType: precipType,
			Intensity:         intensity,
			PrecipitationMM:   item.PrecipitationMM,
			RainMM:            item.RainMM,
			SnowMM:            item.SnowMM,
			ForecastTimestamp: item.ForecastTimestamp,
			ProviderTimestamp: item.ProviderTimestamp,
			RetrievedAt:       now,
			Freshness:         weatherFreshnessFresh,
		})
	}

	weatherCachePut(cacheKey, response, now)
	return response
}

func normalizeWeatherInput(input WeatherPlanningInput) (WeatherPlanningInput, error) {
	maxViewportSpan := config.Data.Weather.MaxViewportSpan
	if maxViewportSpan <= 0 {
		maxViewportSpan = 30
	}
	if input.Zoom < 0 || input.Zoom > 22 {
		return input, fmt.Errorf("zoom out of range")
	}
	if input.ForecastWindowH <= 0 {
		input.ForecastWindowH = weatherMaxForecastHours()
	}
	if input.ForecastDayOffset < 0 {
		input.ForecastDayOffset = 0
	}
	if input.ForecastDayOffset > 7 {
		return input, fmt.Errorf("forecast day offset too large")
	}
	minHoursRequired := (input.ForecastDayOffset + 1) * 24
	if input.ForecastWindowH < minHoursRequired {
		input.ForecastWindowH = minHoursRequired
	}
	if input.ForecastWindowH > weatherMaxForecastHours() {
		return input, fmt.Errorf("forecast window too large")
	}
	if input.MinLat < -90 || input.MaxLat > 90 || input.MinLon < -180 || input.MaxLon > 180 {
		return input, fmt.Errorf("bounds out of range")
	}
	if input.MinLat > input.MaxLat || input.MinLon > input.MaxLon {
		return input, fmt.Errorf("bounds inverted")
	}
	if (input.MaxLat-input.MinLat) > maxViewportSpan || (input.MaxLon-input.MinLon) > maxViewportSpan {
		return input, fmt.Errorf("bounds too wide")
	}
	if input.CenterLat < -90 || input.CenterLat > 90 || input.CenterLon < -180 || input.CenterLon > 180 {
		return input, fmt.Errorf("center out of range")
	}
	return input, nil
}

func weatherMaxForecastHours() int {
	value := config.Data.Weather.MaxForecastHours
	if value <= 0 {
		return 192
	}
	return value
}

func weatherCacheTTL() time.Duration {
	ttl := config.Data.Weather.CacheTTLSeconds
	if ttl <= 0 {
		ttl = 600
	}
	return time.Duration(ttl) * time.Second
}

func weatherStaleTTL() time.Duration {
	ttl := config.Data.Weather.StaleTTLSeconds
	if ttl <= 0 {
		ttl = 3600
	}
	return time.Duration(ttl) * time.Second
}

func weatherPointStep() float64 {
	value := config.Data.Weather.MaxViewportPointStep
	if value <= 0 {
		return 0.2
	}
	return value
}

func weatherCacheKey(input WeatherPlanningInput) string {
	return fmt.Sprintf("%.3f:%.3f:%.3f:%.3f:%.1f:%d:%d",
		input.MinLat,
		input.MaxLat,
		input.MinLon,
		input.MaxLon,
		input.Zoom,
		input.ForecastWindowH,
		input.ForecastDayOffset,
	)
}

func weatherCacheGet(key string) (weatherCacheEntry, bool) {
	weatherCacheMu.RLock()
	defer weatherCacheMu.RUnlock()
	entry, ok := weatherCacheData[key]
	return entry, ok
}

func weatherCachePut(key string, response WeatherPlanningResponse, now time.Time) {
	weatherCacheMu.Lock()
	defer weatherCacheMu.Unlock()
	weatherCacheData[key] = weatherCacheEntry{
		Response:    response,
		StoredAtUTC: now,
	}
}

func weatherDegradedFromCacheOrUnavailable(cacheKey string, input WeatherPlanningInput, now time.Time, errCode string, errMessage string) WeatherPlanningResponse {
	if entry, ok := weatherCacheGet(cacheKey); ok {
		age := now.Sub(entry.StoredAtUTC)
		if age <= weatherStaleTTL() {
			response := entry.Response
			response.GeneratedAt = now
			response.ErrorCode = errCode
			response.ErrorMessage = sanitizeWeatherErrorMessage(errMessage)
			response.Freshness.Status = weatherFreshnessStale
			response.Freshness.Stale = true
			response.Freshness.Degraded = true
			response.Freshness.Source = weatherSourceCache
			response.Freshness.CacheHit = true
			response.Freshness.AgeSeconds = int64(age.Seconds())
			for i := range response.Items {
				response.Items[i].Freshness = weatherFreshnessStale
			}
			return response
		}
	}
	return WeatherPlanningResponse{
		Viewport: WeatherViewportBounds{
			MinLat: input.MinLat,
			MaxLat: input.MaxLat,
			MinLon: input.MinLon,
			MaxLon: input.MaxLon,
			Zoom:   input.Zoom,
		},
		Items:        []WeatherOverlayPoint{},
		Provider:     strings.TrimSpace(config.Data.Weather.Provider),
		GeneratedAt:  now,
		ErrorCode:    errCode,
		ErrorMessage: sanitizeWeatherErrorMessage(errMessage),
		Freshness: WeatherFreshnessMeta{
			Status:     weatherFreshnessUnavailable,
			Stale:      true,
			Degraded:   true,
			Source:     weatherSourceDegraded,
			CacheHit:   false,
			AgeSeconds: 0,
		},
	}
}

func sanitizeWeatherErrorMessage(value string) string {
	output := strings.TrimSpace(value)
	if len(output) > 256 {
		output = output[:256]
	}
	return output
}

func classifyPrecipitation(rainMM float64, snowMM float64, precipitationMM float64) (string, string) {
	if snowMM > 0 {
		switch {
		case snowMM >= 1:
			return "snow", "heavy"
		case snowMM >= 0.2:
			return "snow", "moderate"
		default:
			return "snow", "light"
		}
	}
	if rainMM > 0 || precipitationMM > 0 {
		rainValue := rainMM
		if rainValue == 0 {
			rainValue = precipitationMM
		}
		switch {
		case rainValue >= 4:
			return "rain", "heavy"
		case rainValue >= 0.5:
			return "rain", "moderate"
		default:
			return "rain", "light"
		}
	}
	return "none", "none"
}

func weatherRecordCacheAudit(operation string, status string, now time.Time) {
	_ = createExternalAPIAuditEventFn(ExternalAPIAuditWriteInput{
		Provider:    ExternalAPIProviderOpenMeteo,
		Operation:   strings.TrimSpace(operation),
		StatusClass: strings.TrimSpace(status),
		LatencyMS:   0,
		RequestTime: now,
		CompletedAt: &now,
	})
}

type openMeteoForecastResponse struct {
	GenerationTimeMS float64 `json:"generationtime_ms"`
	HourlyUnits      struct {
		Time          string `json:"time"`
		Precipitation string `json:"precipitation"`
		Rain          string `json:"rain"`
		Snowfall      string `json:"snowfall"`
	} `json:"hourly_units"`
	Hourly struct {
		Time          []string  `json:"time"`
		Precipitation []float64 `json:"precipitation"`
		Rain          []float64 `json:"rain"`
		Snowfall      []float64 `json:"snowfall"`
	} `json:"hourly"`
}

func fetchOpenMeteoPlanningForecast(input WeatherPlanningInput) (weatherProviderResult, error) {
	centerLat := (input.MinLat + input.MaxLat) / 2
	centerLon := (input.MinLon + input.MaxLon) / 2
	baseURL := strings.TrimSpace(config.Data.Weather.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.open-meteo.com"
	}
	providerURL := fmt.Sprintf("%s/v1/forecast?latitude=%f&longitude=%f&hourly=precipitation,rain,snowfall&forecast_hours=%d&timezone=UTC",
		strings.TrimRight(baseURL, "/"),
		centerLat,
		centerLon,
		input.ForecastWindowH,
	)

	if _, err := url.ParseRequestURI(providerURL); err != nil {
		return weatherProviderResult{}, err
	}

	body, err := GetRequestWithExternalAPIAudit(
		ExternalAPIProviderOpenMeteo,
		"planning_forecast",
		providerURL,
		func(raw []byte) error {
			tmp := openMeteoForecastResponse{}
			if decodeErr := json.Unmarshal(raw, &tmp); decodeErr != nil {
				return fmt.Errorf("decode weather payload: %w", decodeErr)
			}
			return nil
		},
	)
	if err != nil {
		return weatherProviderResult{}, err
	}

	output := openMeteoForecastResponse{}
	if err := json.Unmarshal(body, &output); err != nil {
		return weatherProviderResult{}, err
	}

	if len(output.Hourly.Time) == 0 {
		return weatherProviderResult{Points: []weatherProviderPoint{}, ProviderTimestamp: weatherNowFn()}, nil
	}
	now := weatherNowFn()
	targetTime := now.Add(time.Duration(input.ForecastDayOffset) * 24 * time.Hour)
	selectedIndex := selectNearestForecastIndex(output.Hourly.Time, targetTime)
	step := weatherPointStep()
	points := make([]weatherProviderPoint, 0)
	for lat := input.MinLat; lat <= input.MaxLat+0.00001; lat += step {
		for lon := input.MinLon; lon <= input.MaxLon+0.00001; lon += step {
			pointTime, parseErr := time.Parse(time.RFC3339, output.Hourly.Time[selectedIndex])
			if parseErr != nil {
				pointTime = weatherNowFn()
			}
			rain := safeWeatherSeriesValue(output.Hourly.Rain, selectedIndex)
			snow := safeWeatherSeriesValue(output.Hourly.Snowfall, selectedIndex)
			precip := safeWeatherSeriesValue(output.Hourly.Precipitation, selectedIndex)
			points = append(points, weatherProviderPoint{
				Lat:               math.Round(lat*10000) / 10000,
				Lon:               math.Round(lon*10000) / 10000,
				ForecastTimestamp: pointTime.UTC(),
				PrecipitationMM:   precip,
				RainMM:            rain,
				SnowMM:            snow,
				ProviderTimestamp: weatherNowFn(),
			})
		}
	}

	return weatherProviderResult{
		Points:            points,
		ProviderTimestamp: weatherNowFn(),
	}, nil
}

func safeWeatherSeriesValue(values []float64, idx int) float64 {
	if idx < 0 || idx >= len(values) {
		return 0
	}
	return values[idx]
}

func selectNearestForecastIndex(timestamps []string, now time.Time) int {
	if len(timestamps) == 0 {
		return 0
	}
	bestIndex := 0
	bestDiff := time.Duration(1<<63 - 1)
	for i, raw := range timestamps {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			continue
		}
		diff := parsed.Sub(now)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			bestIndex = i
		}
	}
	return bestIndex
}
