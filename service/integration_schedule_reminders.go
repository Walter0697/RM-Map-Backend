package service

import (
	"errors"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

const reminderScheduleQueryWindow = 48 * time.Hour
const reminderDueWindowMinutes = 60

var ErrUserNotInAPIKeyRelation = errors.New("username does not belong to api key relation")

type integrationDueReminderItem struct {
	Schedule       dbmodel.Schedule
	User           dbmodel.User
	ReminderTime   string
	MarkerTimezone string
	LocalDate      string
	LocalNow       time.Time
	ReminderAt     time.Time
}

func isInReminderWindow(localNow time.Time, reminderHour, reminderMinute int) bool {
	reminderWindowStart := reminderHour*60 + reminderMinute
	currentMinute := localNow.Hour()*60 + localNow.Minute()
	reminderWindowEnd := (reminderWindowStart + reminderDueWindowMinutes - 1) % (24 * 60)

	if reminderWindowStart <= reminderWindowEnd {
		return currentMinute >= reminderWindowStart && currentMinute <= reminderWindowEnd
	}
	return currentMinute >= reminderWindowStart || currentMinute <= reminderWindowEnd
}

var integrationReminderNowFn = func() time.Time { return time.Now().UTC() }

func relationContainsUser(relation dbmodel.UserRelation, userID uint) bool {
	return relation.UserOneUID == userID || relation.UserTwoUID == userID
}

func getDueScheduleRemindersByUsername(relation dbmodel.UserRelation, username string) ([]integrationDueReminderItem, int, bool, string, string, string, error) {
	user, reminderTime, _, err := GetUserReminderTime(username)
	if err != nil {
		return nil, 0, false, "", "", "", err
	}
	if !relationContainsUser(relation, user.ID) {
		return nil, 0, false, reminderTime, "", "", ErrUserNotInAPIKeyRelation
	}

	parts := strings.Split(reminderTime, ":")
	reminderHour := 9
	reminderMinute := 0
	if len(parts) == 2 {
		if parsedHour, convErr := time.Parse("15:04", reminderTime); convErr == nil {
			reminderHour = parsedHour.Hour()
			reminderMinute = parsedHour.Minute()
		}
	}

	nowUTC := integrationReminderNowFn().UTC()
	fromUTC := nowUTC.Add(-reminderScheduleQueryWindow)
	toUTC := nowUTC.Add(reminderScheduleQueryWindow)

	schedules := make([]dbmodel.Schedule, 0)
	if err := database.Connection.
		Model(&dbmodel.Schedule{}).
		Preload("SelectedMarker").
		Where("relation_id = ?", relation.ID).
		Where("marker_id IS NOT NULL").
		Where("selected_date >= ? AND selected_date <= ?", fromUTC, toUTC).
		Order("selected_date asc").
		Find(&schedules).Error; err != nil {
		return nil, 0, false, reminderTime, "", "", err
	}

	candidates := make([]integrationDueReminderItem, 0, len(schedules))
	todayScheduleItemsCount := 0
	reminderWindowIsActive := false
	markerLocationCurrentTime := ""
	markerLocationTimezone := ""
	for _, schedule := range schedules {
		if schedule.SelectedMarker == nil {
			continue
		}

		timezone := resolveScheduleTimezone(schedule)
		location, locationErr := time.LoadLocation(timezone)
		if locationErr != nil {
			location = time.UTC
			timezone = "UTC"
		}

		localNow := nowUTC.In(location)
		localSchedule := schedule.SelectedDate.In(location)
		if localNow.Format("2006-01-02") != localSchedule.Format("2006-01-02") {
			continue
		}
		todayScheduleItemsCount++
		if markerLocationCurrentTime == "" {
			markerLocationTimezone = timezone
			markerLocationCurrentTime = localNow.Format(time.RFC3339)
		}

		reminderAt := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), reminderHour, reminderMinute, 0, 0, location)
		if !isInReminderWindow(localNow, reminderHour, reminderMinute) {
			continue
		}
		reminderWindowIsActive = true

		candidates = append(candidates, integrationDueReminderItem{
			Schedule:       schedule,
			User:           *user,
			ReminderTime:   reminderTime,
			MarkerTimezone: timezone,
			LocalDate:      localNow.Format("2006-01-02"),
			LocalNow:       localNow,
			ReminderAt:     reminderAt,
		})
	}

	if len(candidates) == 0 {
		return []integrationDueReminderItem{}, todayScheduleItemsCount, reminderWindowIsActive, reminderTime, markerLocationCurrentTime, markerLocationTimezone, nil
	}

	due := make([]integrationDueReminderItem, 0, len(candidates))
	for _, item := range candidates {
		log := dbmodel.ScheduleReminderDispatchLog{
			UserID:         item.User.ID,
			RelationID:     relation.ID,
			ScheduleID:     item.Schedule.ID,
			LocalDate:      item.LocalDate,
			ReminderTime:   item.ReminderTime,
			MarkerTimezone: item.MarkerTimezone,
			DispatchedAt:   nowUTC,
		}
		result := database.Connection.
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "user_id"},
					{Name: "schedule_id"},
					{Name: "local_date"},
				},
				DoNothing: true,
			}).
			Create(&log)
		if result.Error != nil {
			return nil, 0, false, reminderTime, markerLocationCurrentTime, markerLocationTimezone, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		due = append(due, item)
	}

	return due, todayScheduleItemsCount, reminderWindowIsActive, reminderTime, markerLocationCurrentTime, markerLocationTimezone, nil
}
