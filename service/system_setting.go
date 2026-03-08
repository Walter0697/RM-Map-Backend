package service

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
)

const (
	SystemSettingKeyIOSShortcutInstallURL                   = "ios_shortcut_install_url"
	SystemSettingKeyScheduleTravelEasyThresholdMinutes      = "schedule_travel_easy_threshold_minutes"
	SystemSettingKeyScheduleTravelDifficultThresholdMinutes = "schedule_travel_difficult_threshold_minutes"
	SystemSettingKeyCalendarSyncShortMinutes                = "calendar_sync_short_minutes"
	SystemSettingKeyCalendarSyncMediumMinutes               = "calendar_sync_medium_minutes"
	SystemSettingKeyCalendarSyncLongMinutes                 = "calendar_sync_long_minutes"
	SystemSettingKeyCalendarSyncAutoMinutes                 = "calendar_sync_auto_minutes"

	defaultScheduleTravelEasyThresholdMinutes      = 20
	defaultScheduleTravelDifficultThresholdMinutes = 45
	defaultCalendarSyncShortMinutes                = 30
	defaultCalendarSyncMediumMinutes               = 60
	defaultCalendarSyncLongMinutes                 = 120
	defaultCalendarSyncAutoMinutes                 = 30
)

var ErrInvalidIOSShortcutInstallURL = errors.New("ios_shortcut_install_url must be a valid absolute http or https URL")
var ErrInvalidScheduleTravelThreshold = errors.New("schedule travel thresholds must be positive integers and easy threshold must be less than difficult threshold")
var ErrInvalidCalendarSyncDurations = errors.New("calendar sync durations must be positive integers")

func ValidateIOSShortcutInstallURL(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", false
	}
	return trimmed, true
}

func GetIOSShortcutInstallURL() (string, bool, error) {
	setting := dbmodel.SystemSetting{Key: SystemSettingKeyIOSShortcutInstallURL}
	if err := setting.GetByKey(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}

	validated, ok := ValidateIOSShortcutInstallURL(setting.Value)
	if !ok {
		return "", false, nil
	}
	return validated, true, nil
}

func SetIOSShortcutInstallURL(raw string) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		validated, ok := ValidateIOSShortcutInstallURL(trimmed)
		if !ok {
			return "", false, ErrInvalidIOSShortcutInstallURL
		}
		trimmed = validated
	}

	setting := dbmodel.SystemSetting{
		Key:   SystemSettingKeyIOSShortcutInstallURL,
		Value: trimmed,
	}
	if err := setting.UpsertByKey(database.Connection); err != nil {
		return "", false, err
	}

	if trimmed == "" {
		return "", false, nil
	}
	return trimmed, true, nil
}

type ScheduleTravelThresholds struct {
	EasyThresholdMinutes      int `json:"easy_threshold_minutes"`
	DifficultThresholdMinutes int `json:"difficult_threshold_minutes"`
}

func ValidateScheduleTravelThresholds(easyThresholdMinutes int, difficultThresholdMinutes int) bool {
	if easyThresholdMinutes <= 0 || difficultThresholdMinutes <= 0 {
		return false
	}
	return easyThresholdMinutes < difficultThresholdMinutes
}

func GetScheduleTravelThresholds() (ScheduleTravelThresholds, error) {
	thresholds := defaultScheduleTravelThresholds()

	easyThresholdMinutes, foundEasyThreshold, err := getSystemSettingInt(SystemSettingKeyScheduleTravelEasyThresholdMinutes)
	if err != nil {
		return thresholds, err
	}
	if foundEasyThreshold {
		thresholds.EasyThresholdMinutes = easyThresholdMinutes
	}

	difficultThresholdMinutes, foundDifficultThreshold, err := getSystemSettingInt(SystemSettingKeyScheduleTravelDifficultThresholdMinutes)
	if err != nil {
		return thresholds, err
	}
	if foundDifficultThreshold {
		thresholds.DifficultThresholdMinutes = difficultThresholdMinutes
	}

	if !ValidateScheduleTravelThresholds(thresholds.EasyThresholdMinutes, thresholds.DifficultThresholdMinutes) {
		return thresholds, ErrInvalidScheduleTravelThreshold
	}
	return thresholds, nil
}

func SetScheduleTravelThresholds(easyThresholdMinutes int, difficultThresholdMinutes int) (ScheduleTravelThresholds, error) {
	if !ValidateScheduleTravelThresholds(easyThresholdMinutes, difficultThresholdMinutes) {
		return ScheduleTravelThresholds{}, ErrInvalidScheduleTravelThreshold
	}

	err := database.Connection.Transaction(func(tx *gorm.DB) error {
		easyThreshold := dbmodel.SystemSetting{
			Key:   SystemSettingKeyScheduleTravelEasyThresholdMinutes,
			Value: strconv.Itoa(easyThresholdMinutes),
		}
		if err := easyThreshold.UpsertByKey(tx); err != nil {
			return err
		}

		difficultThreshold := dbmodel.SystemSetting{
			Key:   SystemSettingKeyScheduleTravelDifficultThresholdMinutes,
			Value: strconv.Itoa(difficultThresholdMinutes),
		}
		if err := difficultThreshold.UpsertByKey(tx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ScheduleTravelThresholds{}, err
	}

	return ScheduleTravelThresholds{
		EasyThresholdMinutes:      easyThresholdMinutes,
		DifficultThresholdMinutes: difficultThresholdMinutes,
	}, nil
}

func getSystemSettingInt(key string) (int, bool, error) {
	setting := dbmodel.SystemSetting{Key: key}
	if err := setting.GetByKey(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}

	parsedValue, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil {
		return 0, false, err
	}
	return parsedValue, true, nil
}

func defaultScheduleTravelThresholds() ScheduleTravelThresholds {
	easyThresholdMinutes := config.Data.ScheduleTravel.EasyThresholdMinutes
	if easyThresholdMinutes <= 0 {
		easyThresholdMinutes = defaultScheduleTravelEasyThresholdMinutes
	}

	difficultThresholdMinutes := config.Data.ScheduleTravel.DifficultThresholdMinutes
	if difficultThresholdMinutes <= 0 {
		difficultThresholdMinutes = defaultScheduleTravelDifficultThresholdMinutes
	}
	if difficultThresholdMinutes <= easyThresholdMinutes {
		difficultThresholdMinutes = defaultScheduleTravelDifficultThresholdMinutes
	}

	return ScheduleTravelThresholds{
		EasyThresholdMinutes:      easyThresholdMinutes,
		DifficultThresholdMinutes: difficultThresholdMinutes,
	}
}

type CalendarSyncDurations struct {
	ShortMinutes  int `json:"short_minutes"`
	MediumMinutes int `json:"medium_minutes"`
	LongMinutes   int `json:"long_minutes"`
	AutoMinutes   int `json:"auto_minutes"`
}

func ValidateCalendarSyncDurations(shortMinutes int, mediumMinutes int, longMinutes int, autoMinutes int) bool {
	return shortMinutes > 0 && mediumMinutes > 0 && longMinutes > 0 && autoMinutes > 0
}

func GetCalendarSyncDurations() (CalendarSyncDurations, error) {
	durations := defaultCalendarSyncDurations()
	if database.Connection == nil {
		return durations, nil
	}

	shortMinutes, foundShort, err := getSystemSettingInt(SystemSettingKeyCalendarSyncShortMinutes)
	if err != nil {
		return durations, err
	}
	if foundShort {
		durations.ShortMinutes = shortMinutes
	}

	mediumMinutes, foundMedium, err := getSystemSettingInt(SystemSettingKeyCalendarSyncMediumMinutes)
	if err != nil {
		return durations, err
	}
	if foundMedium {
		durations.MediumMinutes = mediumMinutes
	}

	longMinutes, foundLong, err := getSystemSettingInt(SystemSettingKeyCalendarSyncLongMinutes)
	if err != nil {
		return durations, err
	}
	if foundLong {
		durations.LongMinutes = longMinutes
	}

	autoMinutes, foundAuto, err := getSystemSettingInt(SystemSettingKeyCalendarSyncAutoMinutes)
	if err != nil {
		return durations, err
	}
	if foundAuto {
		durations.AutoMinutes = autoMinutes
	}

	if !ValidateCalendarSyncDurations(durations.ShortMinutes, durations.MediumMinutes, durations.LongMinutes, durations.AutoMinutes) {
		return durations, ErrInvalidCalendarSyncDurations
	}
	return durations, nil
}

func SetCalendarSyncDurations(shortMinutes int, mediumMinutes int, longMinutes int, autoMinutes int) (CalendarSyncDurations, error) {
	if !ValidateCalendarSyncDurations(shortMinutes, mediumMinutes, longMinutes, autoMinutes) {
		return CalendarSyncDurations{}, ErrInvalidCalendarSyncDurations
	}

	err := database.Connection.Transaction(func(tx *gorm.DB) error {
		items := []dbmodel.SystemSetting{
			{Key: SystemSettingKeyCalendarSyncShortMinutes, Value: strconv.Itoa(shortMinutes)},
			{Key: SystemSettingKeyCalendarSyncMediumMinutes, Value: strconv.Itoa(mediumMinutes)},
			{Key: SystemSettingKeyCalendarSyncLongMinutes, Value: strconv.Itoa(longMinutes)},
			{Key: SystemSettingKeyCalendarSyncAutoMinutes, Value: strconv.Itoa(autoMinutes)},
		}
		for _, item := range items {
			if err := item.UpsertByKey(tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return CalendarSyncDurations{}, err
	}

	return CalendarSyncDurations{
		ShortMinutes:  shortMinutes,
		MediumMinutes: mediumMinutes,
		LongMinutes:   longMinutes,
		AutoMinutes:   autoMinutes,
	}, nil
}

func defaultCalendarSyncDurations() CalendarSyncDurations {
	return CalendarSyncDurations{
		ShortMinutes:  defaultCalendarSyncShortMinutes,
		MediumMinutes: defaultCalendarSyncMediumMinutes,
		LongMinutes:   defaultCalendarSyncLongMinutes,
		AutoMinutes:   defaultCalendarSyncAutoMinutes,
	}
}
