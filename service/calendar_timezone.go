package service

import (
	"mapmarker/backend/database/dbmodel"
	"strings"

	"github.com/zsefvlol/timezonemapper"
)

const calendarDefaultTimezone = "UTC"

func resolveScheduleTimezone(schedule dbmodel.Schedule) string {
	if schedule.SelectedMarker == nil {
		return calendarDefaultTimezone
	}
	lat := schedule.SelectedMarker.Latitude
	lon := schedule.SelectedMarker.Longitude
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return calendarDefaultTimezone
	}
	timezone := strings.TrimSpace(timezonemapper.LatLngToTimezoneString(lat, lon))
	if timezone == "" {
		return calendarDefaultTimezone
	}
	return timezone
}
