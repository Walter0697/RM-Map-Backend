package service

import (
	"errors"
	"net/url"
	"strings"

	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
)

const (
	SystemSettingKeyIOSShortcutInstallURL = "ios_shortcut_install_url"
)

var ErrInvalidIOSShortcutInstallURL = errors.New("ios_shortcut_install_url must be a valid absolute http or https URL")

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
