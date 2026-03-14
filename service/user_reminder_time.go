package service

import (
	"fmt"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
	"strconv"
	"strings"
)

const defaultReminderTime = "09:00"

func normalizeReminderTime(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("reminder time is required")
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("reminder time must be HH:MM (24-hour)")
	}

	hour, hourErr := strconv.Atoi(parts[0])
	minute, minuteErr := strconv.Atoi(parts[1])
	if hourErr != nil || minuteErr != nil {
		return "", fmt.Errorf("reminder time must be HH:MM (24-hour)")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return "", fmt.Errorf("reminder time must be within 00:00 to 23:59")
	}

	return fmt.Sprintf("%02d:%02d", hour, minute), nil
}

func resolveReminderTimeOrDefault(raw *string) string {
	if raw == nil {
		return defaultReminderTime
	}
	value, err := normalizeReminderTime(*raw)
	if err != nil {
		return defaultReminderTime
	}
	return value
}

func GetUserReminderTime(username string) (*dbmodel.User, string, string, error) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return nil, "", "", fmt.Errorf("username is required")
	}

	var user dbmodel.User
	user.Username = trimmedUsername
	if err := user.GetUserByUsername(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return nil, "", "", ErrUnknownUsername
		}
		return nil, "", "", err
	}

	preference := dbmodel.UserPreference{UserId: user.ID}
	if err := preference.GetByUserId(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return &user, defaultReminderTime, "default", nil
		}
		return nil, "", "", err
	}

	if preference.PreferredReminderTime == nil {
		return &user, defaultReminderTime, "default", nil
	}

	value, err := normalizeReminderTime(*preference.PreferredReminderTime)
	if err != nil {
		return &user, defaultReminderTime, "default", nil
	}
	return &user, value, "user", nil
}

func SetUserReminderTime(username string, reminderTime string, actor *dbmodel.User) (*dbmodel.UserPreference, string, error) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return nil, "", fmt.Errorf("username is required")
	}

	normalized, err := normalizeReminderTime(reminderTime)
	if err != nil {
		return nil, "", err
	}

	var user dbmodel.User
	user.Username = trimmedUsername
	if err := user.GetUserByUsername(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return nil, "", ErrUnknownUsername
		}
		return nil, "", err
	}

	preference := dbmodel.UserPreference{CurrentUser: user}
	if err := preference.GetOrCreateByUserId(database.Connection); err != nil {
		return nil, "", err
	}

	preference.PreferredReminderTime = &normalized
	_ = actor
	if err := preference.Update(database.Connection); err != nil {
		return nil, "", err
	}
	return &preference, normalized, nil
}
