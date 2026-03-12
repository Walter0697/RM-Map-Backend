package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	scheduleWeatherStatusFresh       = "fresh"
	scheduleWeatherStatusUnavailable = "unavailable"
	scheduleWeatherSourceProvider    = "provider"
	scheduleWeatherSourceDegraded    = "degraded"

	scheduleWeatherReasonFeatureDisabled = "feature_disabled"
	scheduleWeatherReasonMissingLocation = "missing_location"
	scheduleWeatherReasonHorizonExceeded = "forecast_horizon_exceeded"
	scheduleWeatherReasonProviderTimeout = "provider_timeout"
	scheduleWeatherReasonProviderFailure = "provider_failure"
	scheduleWeatherReasonNoForecast      = "forecast_unavailable"

	defaultScheduleWeatherBaseURL            = "https://api.open-meteo.com"
	defaultScheduleWeatherTimeoutMS          = 2500
	defaultScheduleWeatherForecastHorizonDay = 16
)

type scheduleWeatherSnapshot struct {
	Condition         *string
	ForecastAt        *time.Time
	Temperature       *float64
	TemperatureUnit   *string
	Source            string
	Status            string
	UnavailableReason *string
	FetchedAt         time.Time
}

type scheduleWeatherProviderSnapshot struct {
	Condition       string
	ForecastAt      time.Time
	Temperature     *float64
	TemperatureUnit *string
}

type scheduleWeatherProviderResponse struct {
	Hourly struct {
		Time        []string  `json:"time"`
		Temperature []float64 `json:"temperature_2m"`
		WeatherCode []int     `json:"weather_code"`
	} `json:"hourly"`
	HourlyUnits struct {
		Temperature string `json:"temperature_2m"`
	} `json:"hourly_units"`
}

var scheduleWeatherNowFn = func() time.Time { return time.Now().UTC() }
var scheduleWeatherFetchProviderFn = fetchScheduleWeatherFromOpenMeteo

func resolveScheduleWeatherSnapshot(marker *dbmodel.Marker, selectedDate time.Time) scheduleWeatherSnapshot {
	fetchedAt := scheduleWeatherNowFn().UTC()

	if !config.Data.Weather.Enable {
		return unavailableScheduleWeatherSnapshot(fetchedAt, scheduleWeatherReasonFeatureDisabled)
	}
	if marker == nil {
		return unavailableScheduleWeatherSnapshot(fetchedAt, scheduleWeatherReasonMissingLocation)
	}

	selectedUTC := selectedDate.UTC()
	horizonEnd := fetchedAt.AddDate(0, 0, resolveScheduleWeatherForecastHorizonDays())
	if selectedUTC.After(horizonEnd) {
		return unavailableScheduleWeatherSnapshot(fetchedAt, scheduleWeatherReasonHorizonExceeded)
	}

	snapshot, err := scheduleWeatherFetchProviderFn(marker.Latitude, marker.Longitude, selectedUTC)
	if err != nil {
		return unavailableScheduleWeatherSnapshot(fetchedAt, scheduleWeatherErrorReason(err))
	}

	return scheduleWeatherSnapshot{
		Condition:       ptrString(strings.TrimSpace(snapshot.Condition)),
		ForecastAt:      ptrTime(snapshot.ForecastAt.UTC()),
		Temperature:     snapshot.Temperature,
		TemperatureUnit: snapshot.TemperatureUnit,
		Source:          scheduleWeatherSourceProvider,
		Status:          scheduleWeatherStatusFresh,
		FetchedAt:       fetchedAt,
	}
}

func applyScheduleWeatherSnapshot(schedule *dbmodel.Schedule, snapshot scheduleWeatherSnapshot) {
	schedule.WeatherCondition = snapshot.Condition
	schedule.WeatherForecastAt = snapshot.ForecastAt
	schedule.WeatherTemperature = snapshot.Temperature
	schedule.WeatherTemperatureUnit = snapshot.TemperatureUnit
	schedule.WeatherSource = ptrString(snapshot.Source)
	schedule.WeatherStatus = ptrString(snapshot.Status)
	schedule.WeatherUnavailableReason = snapshot.UnavailableReason
	schedule.WeatherFetchedAt = ptrTime(snapshot.FetchedAt)
}

func unavailableScheduleWeatherSnapshot(fetchedAt time.Time, reason string) scheduleWeatherSnapshot {
	return scheduleWeatherSnapshot{
		Source:            scheduleWeatherSourceDegraded,
		Status:            scheduleWeatherStatusUnavailable,
		UnavailableReason: ptrString(strings.TrimSpace(reason)),
		FetchedAt:         fetchedAt,
	}
}

func fetchScheduleWeatherFromOpenMeteo(lat float64, lon float64, selectedDateUTC time.Time) (scheduleWeatherProviderSnapshot, error) {
	baseURL := strings.TrimSpace(config.Data.Weather.BaseURL)
	if baseURL == "" {
		baseURL = defaultScheduleWeatherBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	endpoint, err := url.Parse(baseURL + "/v1/forecast")
	if err != nil {
		return scheduleWeatherProviderSnapshot{}, err
	}
	day := selectedDateUTC.Format("2006-01-02")
	query := endpoint.Query()
	query.Set("latitude", fmt.Sprintf("%.6f", lat))
	query.Set("longitude", fmt.Sprintf("%.6f", lon))
	query.Set("hourly", "weather_code,temperature_2m")
	query.Set("timezone", "UTC")
	query.Set("start_date", day)
	query.Set("end_date", day)
	endpoint.RawQuery = query.Encode()

	timeout := time.Duration(resolveScheduleWeatherTimeoutMS()) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return scheduleWeatherProviderSnapshot{}, err
	}

	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		return scheduleWeatherProviderSnapshot{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return scheduleWeatherProviderSnapshot{}, fmt.Errorf("status code %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return scheduleWeatherProviderSnapshot{}, err
	}

	payload := scheduleWeatherProviderResponse{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return scheduleWeatherProviderSnapshot{}, fmt.Errorf("decode weather payload: %w", err)
	}

	idx := closestScheduleWeatherIndex(payload.Hourly.Time, selectedDateUTC)
	if idx < 0 {
		return scheduleWeatherProviderSnapshot{}, fmt.Errorf(scheduleWeatherReasonNoForecast)
	}

	condition := mapOpenMeteoWeatherCodeToCondition(payload.Hourly.WeatherCode[idx])
	forecastAt, parseErr := parseOpenMeteoTimestamp(payload.Hourly.Time[idx])
	if parseErr != nil {
		forecastAt = selectedDateUTC
	}

	var temperature *float64
	if idx < len(payload.Hourly.Temperature) {
		temperature = ptrFloat(payload.Hourly.Temperature[idx])
	}

	temperatureUnit := ptrString(strings.TrimSpace(payload.HourlyUnits.Temperature))
	if temperatureUnit != nil && *temperatureUnit == "" {
		temperatureUnit = nil
	}

	return scheduleWeatherProviderSnapshot{
		Condition:       condition,
		ForecastAt:      forecastAt.UTC(),
		Temperature:     temperature,
		TemperatureUnit: temperatureUnit,
	}, nil
}

func closestScheduleWeatherIndex(times []string, target time.Time) int {
	bestIdx := -1
	var bestDiff time.Duration
	for i, raw := range times {
		candidate, err := parseOpenMeteoTimestamp(raw)
		if err != nil {
			continue
		}
		diff := candidate.Sub(target)
		if diff < 0 {
			diff = -diff
		}
		if bestIdx < 0 || diff < bestDiff {
			bestIdx = i
			bestDiff = diff
		}
	}
	return bestIdx
}

func parseOpenMeteoTimestamp(raw string) (time.Time, error) {
	candidate, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return candidate.UTC(), nil
	}

	// Open-Meteo hourly timestamps are typically returned as "2006-01-02T15:04" without timezone.
	candidate, err = time.Parse("2006-01-02T15:04", raw)
	if err == nil {
		return candidate.UTC(), nil
	}

	return time.Time{}, err
}

func mapOpenMeteoWeatherCodeToCondition(code int) string {
	switch code {
	case 71, 73, 75, 77, 85, 86:
		return "snow"
	case 51, 53, 55, 56, 57, 61, 63, 65, 66, 67, 80, 81, 82, 95, 96, 99:
		return "rain"
	case 1, 2, 3, 45, 48:
		return "cloudy"
	default:
		return "clear"
	}
}

func scheduleWeatherErrorReason(err error) string {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout"):
		return scheduleWeatherReasonProviderTimeout
	case strings.Contains(msg, scheduleWeatherReasonNoForecast):
		return scheduleWeatherReasonNoForecast
	default:
		return scheduleWeatherReasonProviderFailure
	}
}

func resolveScheduleWeatherTimeoutMS() int {
	timeoutMS := config.Data.Weather.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = defaultScheduleWeatherTimeoutMS
	}
	return timeoutMS
}

func resolveScheduleWeatherForecastHorizonDays() int {
	horizonDays := defaultScheduleWeatherForecastHorizonDay
	if config.Data.Weather.MaxForecastHours > 0 {
		calculatedDays := config.Data.Weather.MaxForecastHours / 24
		if calculatedDays > horizonDays {
			horizonDays = calculatedDays
		}
	}
	return horizonDays
}

func ptrString(value string) *string {
	cloned := value
	return &cloned
}

func ptrFloat(value float64) *float64 {
	cloned := value
	return &cloned
}

func ptrTime(value time.Time) *time.Time {
	cloned := value
	return &cloned
}
