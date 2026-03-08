package service

import (
	"log"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"
)

func buildCalendarEventRequestFromSchedule(schedule dbmodel.Schedule) CalendarEventUpsertRequest {
	durationMinutes, durationBucket := resolveCalendarSyncDurationMinutes(schedule)
	timezone := resolveScheduleTimezone(schedule)
	location, timezoneErr := time.LoadLocation(timezone)
	if timezoneErr != nil {
		timezone = calendarDefaultTimezone
		location = time.UTC
	}
	localStart := schedule.SelectedDate.In(location)
	request := CalendarEventUpsertRequest{
		Title:       schedule.Label,
		Description: schedule.Description,
		StartAt:     localStart,
		Timezone:    timezone,
	}
	endAt := localStart.Add(time.Duration(durationMinutes) * time.Minute)
	request.EndAt = &endAt
	if schedule.SelectedMarker != nil {
		request.Location = strings.TrimSpace(schedule.SelectedMarker.Label)
	}
	log.Printf(
		"[calendar-sync-duration] schedule_id=%d estimate_time=%q timezone=%s bucket=%s duration_minutes=%d",
		schedule.ID,
		strings.TrimSpace(strings.ToLower(scheduleMarkerEstimateTime(schedule))),
		timezone,
		durationBucket,
		durationMinutes,
	)
	return request
}

func resolveCalendarSyncDurationMinutes(schedule dbmodel.Schedule) (int, string) {
	durations, err := GetCalendarSyncDurations()
	if err != nil {
		log.Printf("[calendar-sync-duration] using default durations due to error=%v", err)
		durations = defaultCalendarSyncDurations()
	}

	bucket := strings.TrimSpace(strings.ToLower(scheduleMarkerEstimateTime(schedule)))
	switch bucket {
	case "short":
		return durations.ShortMinutes, "short"
	case "medium":
		return durations.MediumMinutes, "medium"
	case "long":
		return durations.LongMinutes, "long"
	default:
		return durations.AutoMinutes, "auto"
	}
}

func scheduleMarkerEstimateTime(schedule dbmodel.Schedule) string {
	if schedule.SelectedMarker == nil {
		return ""
	}
	return schedule.SelectedMarker.EstimateTime
}
