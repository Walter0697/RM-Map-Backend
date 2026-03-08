package service

import (
	"mapmarker/backend/database/dbmodel"
	"testing"
)

func TestResolveScheduleTimezone(t *testing.T) {
	t.Run("returns UTC when marker is missing", func(t *testing.T) {
		schedule := dbmodel.Schedule{}
		if got := resolveScheduleTimezone(schedule); got != "UTC" {
			t.Fatalf("expected UTC, got %s", got)
		}
	})

	t.Run("resolves timezone from marker coordinates", func(t *testing.T) {
		schedule := dbmodel.Schedule{
			SelectedMarker: &dbmodel.Marker{
				Latitude:  1.3521,
				Longitude: 103.8198,
			},
		}
		if got := resolveScheduleTimezone(schedule); got != "Asia/Singapore" {
			t.Fatalf("expected Asia/Singapore, got %s", got)
		}
	})

	t.Run("returns UTC for invalid coordinates", func(t *testing.T) {
		schedule := dbmodel.Schedule{
			SelectedMarker: &dbmodel.Marker{
				Latitude:  999,
				Longitude: 999,
			},
		}
		if got := resolveScheduleTimezone(schedule); got != "UTC" {
			t.Fatalf("expected UTC, got %s", got)
		}
	})
}
