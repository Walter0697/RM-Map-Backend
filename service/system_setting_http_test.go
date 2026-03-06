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
