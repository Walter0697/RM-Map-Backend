package service

import (
	"fmt"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"testing"
	"time"
)

func TestResolveScheduleWeatherSnapshotSuccess(t *testing.T) {
	originalNow := scheduleWeatherNowFn
	originalFetch := scheduleWeatherFetchProviderFn
	defer func() {
		scheduleWeatherNowFn = originalNow
		scheduleWeatherFetchProviderFn = originalFetch
	}()

	config.Data.Weather.Enable = true
	config.Data.Weather.TimeoutMS = 1200
	config.Data.Weather.MaxForecastHours = 24

	now := time.Date(2026, time.March, 12, 8, 0, 0, 0, time.UTC)
	selectedAt := time.Date(2026, time.March, 12, 18, 30, 0, 0, time.UTC)
	scheduleWeatherNowFn = func() time.Time { return now }
	scheduleWeatherFetchProviderFn = func(lat float64, lon float64, selectedDateUTC time.Time) (scheduleWeatherProviderSnapshot, error) {
		if lat != 22.301 || lon != 114.174 {
			t.Fatalf("unexpected marker coords lat=%v lon=%v", lat, lon)
		}
		if !selectedDateUTC.Equal(selectedAt.UTC()) {
			t.Fatalf("unexpected selected date %s", selectedDateUTC.Format(time.RFC3339))
		}
		temp := 21.5
		unit := "C"
		return scheduleWeatherProviderSnapshot{
			Condition:       "rain",
			ForecastAt:      selectedDateUTC,
			Temperature:     &temp,
			TemperatureUnit: &unit,
		}, nil
	}

	marker := &dbmodel.Marker{Latitude: 22.301, Longitude: 114.174}
	snapshot := resolveScheduleWeatherSnapshot(marker, selectedAt)

	if snapshot.Status != scheduleWeatherStatusFresh {
		t.Fatalf("expected fresh status, got %s", snapshot.Status)
	}
	if snapshot.Source != scheduleWeatherSourceProvider {
		t.Fatalf("expected provider source, got %s", snapshot.Source)
	}
	if snapshot.UnavailableReason != nil {
		t.Fatalf("expected no unavailable reason, got %v", *snapshot.UnavailableReason)
	}
	if snapshot.Condition == nil || *snapshot.Condition != "rain" {
		t.Fatalf("expected rain condition, got %#v", snapshot.Condition)
	}
	if snapshot.Temperature == nil || *snapshot.Temperature != 21.5 {
		t.Fatalf("expected temperature 21.5, got %#v", snapshot.Temperature)
	}
	if snapshot.TemperatureUnit == nil || *snapshot.TemperatureUnit != "C" {
		t.Fatalf("expected temperature unit C, got %#v", snapshot.TemperatureUnit)
	}
}

func TestResolveScheduleWeatherSnapshotTimeoutFallback(t *testing.T) {
	originalNow := scheduleWeatherNowFn
	originalFetch := scheduleWeatherFetchProviderFn
	defer func() {
		scheduleWeatherNowFn = originalNow
		scheduleWeatherFetchProviderFn = originalFetch
	}()

	config.Data.Weather.Enable = true
	config.Data.Weather.TimeoutMS = 100

	now := time.Date(2026, time.March, 12, 8, 0, 0, 0, time.UTC)
	scheduleWeatherNowFn = func() time.Time { return now }
	scheduleWeatherFetchProviderFn = func(lat float64, lon float64, selectedDateUTC time.Time) (scheduleWeatherProviderSnapshot, error) {
		return scheduleWeatherProviderSnapshot{}, fmt.Errorf("context deadline exceeded")
	}

	marker := &dbmodel.Marker{Latitude: 22.301, Longitude: 114.174}
	snapshot := resolveScheduleWeatherSnapshot(marker, now.Add(2*time.Hour))

	if snapshot.Status != scheduleWeatherStatusUnavailable {
		t.Fatalf("expected unavailable status, got %s", snapshot.Status)
	}
	if snapshot.Source != scheduleWeatherSourceDegraded {
		t.Fatalf("expected degraded source, got %s", snapshot.Source)
	}
	if snapshot.UnavailableReason == nil || *snapshot.UnavailableReason != scheduleWeatherReasonProviderTimeout {
		t.Fatalf("expected timeout reason, got %#v", snapshot.UnavailableReason)
	}
}

func TestResolveScheduleWeatherSnapshotHorizonExceeded(t *testing.T) {
	originalNow := scheduleWeatherNowFn
	originalFetch := scheduleWeatherFetchProviderFn
	defer func() {
		scheduleWeatherNowFn = originalNow
		scheduleWeatherFetchProviderFn = originalFetch
	}()

	config.Data.Weather.Enable = true
	config.Data.Weather.MaxForecastHours = 24

	now := time.Date(2026, time.March, 12, 8, 0, 0, 0, time.UTC)
	scheduleWeatherNowFn = func() time.Time { return now }
	scheduleWeatherFetchProviderFn = func(lat float64, lon float64, selectedDateUTC time.Time) (scheduleWeatherProviderSnapshot, error) {
		t.Fatalf("provider fetch should not be called when horizon is exceeded")
		return scheduleWeatherProviderSnapshot{}, nil
	}

	marker := &dbmodel.Marker{Latitude: 22.301, Longitude: 114.174}
	snapshot := resolveScheduleWeatherSnapshot(marker, now.AddDate(0, 0, 20))

	if snapshot.Status != scheduleWeatherStatusUnavailable {
		t.Fatalf("expected unavailable status, got %s", snapshot.Status)
	}
	if snapshot.UnavailableReason == nil || *snapshot.UnavailableReason != scheduleWeatherReasonHorizonExceeded {
		t.Fatalf("expected horizon-exceeded reason, got %#v", snapshot.UnavailableReason)
	}
}

func TestClosestScheduleWeatherIndexOpenMeteoFormat(t *testing.T) {
	target := time.Date(2026, time.March, 12, 13, 30, 0, 0, time.UTC)
	times := []string{
		"2026-03-12T12:00",
		"2026-03-12T13:00",
		"2026-03-12T14:00",
	}

	index := closestScheduleWeatherIndex(times, target)
	if index != 1 {
		t.Fatalf("expected closest index 1, got %d", index)
	}
}
