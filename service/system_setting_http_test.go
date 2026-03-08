package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"mapmarker/backend/database/dbmodel"
)

func TestAdminScheduleTravelThresholdHandlersRequireAdmin(t *testing.T) {
	originalRequireAdmin := systemSettingRequireAdminFn
	originalGetThresholds := systemSettingGetScheduleTravelThresholdsFn
	originalSetThresholds := systemSettingSetScheduleTravelThresholdsFn
	defer func() {
		systemSettingRequireAdminFn = originalRequireAdmin
		systemSettingGetScheduleTravelThresholdsFn = originalGetThresholds
		systemSettingSetScheduleTravelThresholdsFn = originalSetThresholds
	}()

	systemSettingRequireAdminFn = func(http.ResponseWriter, *http.Request) *dbmodel.User {
		return nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/settings/schedule-travel-thresholds", nil)
	AdminGetScheduleTravelThresholdsHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected unchanged status when admin check short-circuits, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/admin/settings/schedule-travel-thresholds", bytes.NewBufferString(`{"easy_threshold_minutes":20,"difficult_threshold_minutes":45}`))
	AdminUpdateScheduleTravelThresholdsHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected unchanged status when admin check short-circuits, got %d", recorder.Code)
	}
}

func TestAdminGetScheduleTravelThresholdsHandlerSuccess(t *testing.T) {
	originalRequireAdmin := systemSettingRequireAdminFn
	originalGetThresholds := systemSettingGetScheduleTravelThresholdsFn
	defer func() {
		systemSettingRequireAdminFn = originalRequireAdmin
		systemSettingGetScheduleTravelThresholdsFn = originalGetThresholds
	}()

	systemSettingRequireAdminFn = func(http.ResponseWriter, *http.Request) *dbmodel.User {
		return &dbmodel.User{Role: "admin"}
	}
	systemSettingGetScheduleTravelThresholdsFn = func() (ScheduleTravelThresholds, error) {
		return ScheduleTravelThresholds{
			EasyThresholdMinutes:      15,
			DifficultThresholdMinutes: 40,
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/settings/schedule-travel-thresholds", nil)
	AdminGetScheduleTravelThresholdsHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); body == "" || !bytes.Contains([]byte(body), []byte(`"easy_threshold_minutes":15`)) || !bytes.Contains([]byte(body), []byte(`"difficult_threshold_minutes":40`)) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestAdminUpdateScheduleTravelThresholdsHandler(t *testing.T) {
	originalRequireAdmin := systemSettingRequireAdminFn
	originalSetThresholds := systemSettingSetScheduleTravelThresholdsFn
	defer func() {
		systemSettingRequireAdminFn = originalRequireAdmin
		systemSettingSetScheduleTravelThresholdsFn = originalSetThresholds
	}()

	systemSettingRequireAdminFn = func(http.ResponseWriter, *http.Request) *dbmodel.User {
		return &dbmodel.User{Role: "admin"}
	}

	t.Run("returns bad request when payload is missing values", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/settings/schedule-travel-thresholds", bytes.NewBufferString(`{"easy_threshold_minutes":20}`))
		AdminUpdateScheduleTravelThresholdsHandler(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
	})

	t.Run("returns bad request when service validation fails", func(t *testing.T) {
		systemSettingSetScheduleTravelThresholdsFn = func(int, int) (ScheduleTravelThresholds, error) {
			return ScheduleTravelThresholds{}, ErrInvalidScheduleTravelThreshold
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/settings/schedule-travel-thresholds", bytes.NewBufferString(`{"easy_threshold_minutes":45,"difficult_threshold_minutes":30}`))
		AdminUpdateScheduleTravelThresholdsHandler(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
	})

	t.Run("returns saved thresholds", func(t *testing.T) {
		systemSettingSetScheduleTravelThresholdsFn = func(easyThresholdMinutes int, difficultThresholdMinutes int) (ScheduleTravelThresholds, error) {
			return ScheduleTravelThresholds{
				EasyThresholdMinutes:      easyThresholdMinutes,
				DifficultThresholdMinutes: difficultThresholdMinutes,
			}, nil
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/settings/schedule-travel-thresholds", bytes.NewBufferString(`{"easy_threshold_minutes":20,"difficult_threshold_minutes":45}`))
		AdminUpdateScheduleTravelThresholdsHandler(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		if body := recorder.Body.String(); body == "" || !bytes.Contains([]byte(body), []byte(`"easy_threshold_minutes":20`)) || !bytes.Contains([]byte(body), []byte(`"difficult_threshold_minutes":45`)) {
			t.Fatalf("unexpected body: %s", body)
		}
	})
}

func TestAdminCalendarSyncDurationHandlersRequireAdmin(t *testing.T) {
	originalRequireAdmin := systemSettingRequireAdminFn
	originalGetDurations := systemSettingGetCalendarSyncDurationsFn
	originalSetDurations := systemSettingSetCalendarSyncDurationsFn
	defer func() {
		systemSettingRequireAdminFn = originalRequireAdmin
		systemSettingGetCalendarSyncDurationsFn = originalGetDurations
		systemSettingSetCalendarSyncDurationsFn = originalSetDurations
	}()

	systemSettingRequireAdminFn = func(http.ResponseWriter, *http.Request) *dbmodel.User {
		return nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/settings/calendar-sync-durations", nil)
	AdminGetCalendarSyncDurationsHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected unchanged status when admin check short-circuits, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/admin/settings/calendar-sync-durations", bytes.NewBufferString(`{"short_minutes":30,"medium_minutes":60,"long_minutes":120,"auto_minutes":30}`))
	AdminUpdateCalendarSyncDurationsHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected unchanged status when admin check short-circuits, got %d", recorder.Code)
	}
}

func TestAdminGetCalendarSyncDurationsHandlerSuccess(t *testing.T) {
	originalRequireAdmin := systemSettingRequireAdminFn
	originalGetDurations := systemSettingGetCalendarSyncDurationsFn
	defer func() {
		systemSettingRequireAdminFn = originalRequireAdmin
		systemSettingGetCalendarSyncDurationsFn = originalGetDurations
	}()

	systemSettingRequireAdminFn = func(http.ResponseWriter, *http.Request) *dbmodel.User {
		return &dbmodel.User{Role: "admin"}
	}
	systemSettingGetCalendarSyncDurationsFn = func() (CalendarSyncDurations, error) {
		return CalendarSyncDurations{
			ShortMinutes:  35,
			MediumMinutes: 75,
			LongMinutes:   140,
			AutoMinutes:   30,
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/settings/calendar-sync-durations", nil)
	AdminGetCalendarSyncDurationsHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if body == "" || !bytes.Contains([]byte(body), []byte(`"short_minutes":35`)) || !bytes.Contains([]byte(body), []byte(`"medium_minutes":75`)) || !bytes.Contains([]byte(body), []byte(`"long_minutes":140`)) || !bytes.Contains([]byte(body), []byte(`"auto_minutes":30`)) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestAdminUpdateCalendarSyncDurationsHandler(t *testing.T) {
	originalRequireAdmin := systemSettingRequireAdminFn
	originalSetDurations := systemSettingSetCalendarSyncDurationsFn
	defer func() {
		systemSettingRequireAdminFn = originalRequireAdmin
		systemSettingSetCalendarSyncDurationsFn = originalSetDurations
	}()

	systemSettingRequireAdminFn = func(http.ResponseWriter, *http.Request) *dbmodel.User {
		return &dbmodel.User{Role: "admin"}
	}

	t.Run("returns bad request when payload is missing values", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/settings/calendar-sync-durations", bytes.NewBufferString(`{"short_minutes":30}`))
		AdminUpdateCalendarSyncDurationsHandler(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
	})

	t.Run("returns bad request when service validation fails", func(t *testing.T) {
		systemSettingSetCalendarSyncDurationsFn = func(int, int, int, int) (CalendarSyncDurations, error) {
			return CalendarSyncDurations{}, ErrInvalidCalendarSyncDurations
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/settings/calendar-sync-durations", bytes.NewBufferString(`{"short_minutes":0,"medium_minutes":60,"long_minutes":120,"auto_minutes":30}`))
		AdminUpdateCalendarSyncDurationsHandler(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", recorder.Code)
		}
	})

	t.Run("returns saved durations", func(t *testing.T) {
		systemSettingSetCalendarSyncDurationsFn = func(short int, medium int, long int, auto int) (CalendarSyncDurations, error) {
			return CalendarSyncDurations{
				ShortMinutes:  short,
				MediumMinutes: medium,
				LongMinutes:   long,
				AutoMinutes:   auto,
			}, nil
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/admin/settings/calendar-sync-durations", bytes.NewBufferString(`{"short_minutes":30,"medium_minutes":60,"long_minutes":120,"auto_minutes":30}`))
		AdminUpdateCalendarSyncDurationsHandler(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		body := recorder.Body.String()
		if body == "" || !bytes.Contains([]byte(body), []byte(`"short_minutes":30`)) || !bytes.Contains([]byte(body), []byte(`"medium_minutes":60`)) || !bytes.Contains([]byte(body), []byte(`"long_minutes":120`)) || !bytes.Contains([]byte(body), []byte(`"auto_minutes":30`)) {
			t.Fatalf("unexpected body: %s", body)
		}
	})
}
