package service

import (
	"encoding/json"
	"net/http"
)

var (
	systemSettingCurrentUserFromRequestFn      = currentUserFromRequest
	systemSettingRequireAdminFn                = requireAdmin
	systemSettingGetIOSShortcutInstallURLFn    = GetIOSShortcutInstallURL
	systemSettingSetIOSShortcutInstallURLFn    = SetIOSShortcutInstallURL
	systemSettingGetScheduleTravelThresholdsFn = GetScheduleTravelThresholds
	systemSettingSetScheduleTravelThresholdsFn = SetScheduleTravelThresholds
	systemSettingGetCalendarSyncDurationsFn    = GetCalendarSyncDurations
	systemSettingSetCalendarSyncDurationsFn    = SetCalendarSyncDurations
)

type updateIOSShortcutInstallURLRequest struct {
	IOSShortcutInstallURL *string `json:"ios_shortcut_install_url"`
}

type iosShortcutInstallURLResponse struct {
	IOSShortcutInstallURL string `json:"ios_shortcut_install_url,omitempty"`
}

type updateScheduleTravelThresholdsRequest struct {
	EasyThresholdMinutes      *int `json:"easy_threshold_minutes"`
	DifficultThresholdMinutes *int `json:"difficult_threshold_minutes"`
}

type scheduleTravelThresholdsResponse struct {
	EasyThresholdMinutes      int `json:"easy_threshold_minutes"`
	DifficultThresholdMinutes int `json:"difficult_threshold_minutes"`
}

type updateCalendarSyncDurationsRequest struct {
	ShortMinutes  *int `json:"short_minutes"`
	MediumMinutes *int `json:"medium_minutes"`
	LongMinutes   *int `json:"long_minutes"`
	AutoMinutes   *int `json:"auto_minutes"`
}

type calendarSyncDurationsResponse struct {
	ShortMinutes  int `json:"short_minutes"`
	MediumMinutes int `json:"medium_minutes"`
	LongMinutes   int `json:"long_minutes"`
	AutoMinutes   int `json:"auto_minutes"`
}

func SettingsGetIOSShortcutInstallURLHandler(w http.ResponseWriter, r *http.Request) {
	user := systemSettingCurrentUserFromRequestFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	shortcutURL, present, err := systemSettingGetIOSShortcutInstallURLFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := iosShortcutInstallURLResponse{}
	if present {
		response.IOSShortcutInstallURL = shortcutURL
	}
	respondJSON(w, http.StatusOK, response)
}

func AdminGetIOSShortcutInstallURLHandler(w http.ResponseWriter, r *http.Request) {
	if systemSettingRequireAdminFn(w, r) == nil {
		return
	}

	shortcutURL, present, err := systemSettingGetIOSShortcutInstallURLFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := iosShortcutInstallURLResponse{}
	if present {
		response.IOSShortcutInstallURL = shortcutURL
	}
	respondJSON(w, http.StatusOK, response)
}

func AdminUpdateIOSShortcutInstallURLHandler(w http.ResponseWriter, r *http.Request) {
	if systemSettingRequireAdminFn(w, r) == nil {
		return
	}

	request := updateIOSShortcutInstallURLRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.IOSShortcutInstallURL == nil {
		http.Error(w, "ios_shortcut_install_url is required", http.StatusBadRequest)
		return
	}

	shortcutURL, present, err := systemSettingSetIOSShortcutInstallURLFn(*request.IOSShortcutInstallURL)
	if err != nil {
		if err == ErrInvalidIOSShortcutInstallURL {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := iosShortcutInstallURLResponse{}
	if present {
		response.IOSShortcutInstallURL = shortcutURL
	}
	respondJSON(w, http.StatusOK, response)
}

func AdminGetScheduleTravelThresholdsHandler(w http.ResponseWriter, r *http.Request) {
	if systemSettingRequireAdminFn(w, r) == nil {
		return
	}

	thresholds, err := systemSettingGetScheduleTravelThresholdsFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, scheduleTravelThresholdsResponse{
		EasyThresholdMinutes:      thresholds.EasyThresholdMinutes,
		DifficultThresholdMinutes: thresholds.DifficultThresholdMinutes,
	})
}

func AdminUpdateScheduleTravelThresholdsHandler(w http.ResponseWriter, r *http.Request) {
	if systemSettingRequireAdminFn(w, r) == nil {
		return
	}

	request := updateScheduleTravelThresholdsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.EasyThresholdMinutes == nil || request.DifficultThresholdMinutes == nil {
		http.Error(w, "easy_threshold_minutes and difficult_threshold_minutes are required", http.StatusBadRequest)
		return
	}

	thresholds, err := systemSettingSetScheduleTravelThresholdsFn(*request.EasyThresholdMinutes, *request.DifficultThresholdMinutes)
	if err != nil {
		if err == ErrInvalidScheduleTravelThreshold {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, scheduleTravelThresholdsResponse{
		EasyThresholdMinutes:      thresholds.EasyThresholdMinutes,
		DifficultThresholdMinutes: thresholds.DifficultThresholdMinutes,
	})
}

func AdminGetCalendarSyncDurationsHandler(w http.ResponseWriter, r *http.Request) {
	if systemSettingRequireAdminFn(w, r) == nil {
		return
	}

	durations, err := systemSettingGetCalendarSyncDurationsFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, calendarSyncDurationsResponse{
		ShortMinutes:  durations.ShortMinutes,
		MediumMinutes: durations.MediumMinutes,
		LongMinutes:   durations.LongMinutes,
		AutoMinutes:   durations.AutoMinutes,
	})
}

func AdminUpdateCalendarSyncDurationsHandler(w http.ResponseWriter, r *http.Request) {
	if systemSettingRequireAdminFn(w, r) == nil {
		return
	}

	request := updateCalendarSyncDurationsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.ShortMinutes == nil || request.MediumMinutes == nil || request.LongMinutes == nil || request.AutoMinutes == nil {
		http.Error(w, "short_minutes, medium_minutes, long_minutes, and auto_minutes are required", http.StatusBadRequest)
		return
	}

	durations, err := systemSettingSetCalendarSyncDurationsFn(*request.ShortMinutes, *request.MediumMinutes, *request.LongMinutes, *request.AutoMinutes)
	if err != nil {
		if err == ErrInvalidCalendarSyncDurations {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, calendarSyncDurationsResponse{
		ShortMinutes:  durations.ShortMinutes,
		MediumMinutes: durations.MediumMinutes,
		LongMinutes:   durations.LongMinutes,
		AutoMinutes:   durations.AutoMinutes,
	})
}
