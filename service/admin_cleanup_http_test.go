package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mapmarker/backend/database/dbmodel"

	"github.com/go-chi/chi"
)

func stubCleanupHooks() {
	cleanupCurrentUserFn = currentUserFromRequest
	cleanupListMarkersFn = func(queryOption integrationListQuery) ([]adminCleanupMarkerResponse, int64, error) {
		return []adminCleanupMarkerResponse{}, 0, nil
	}
	cleanupListSchedulesFn = func(queryOption integrationListQuery) ([]adminCleanupScheduleResponse, int64, error) {
		return []adminCleanupScheduleResponse{}, 0, nil
	}
	cleanupFindMarkerByIDFn = func(id uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: id}},
			Label:      "Marker A",
		}, nil
	}
	cleanupFindScheduleByIDFn = func(id uint) (*dbmodel.Schedule, error) {
		return &dbmodel.Schedule{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: id}},
			Label:      "Schedule A",
		}, nil
	}
	cleanupDeleteMarkerByIDFn = func(id uint) error { return nil }
	cleanupDeleteScheduleByIDFn = func(id uint) error { return nil }
	cleanupValidateMarkerDeletionFn = func(id uint) error { return nil }
	cleanupCreateJobFn = func(request adminCleanupScheduleRequest, executeAt time.Time, actor dbmodel.User) (*adminCleanupJobResponse, error) {
		return &adminCleanupJobResponse{
			ID:           1,
			EntityType:   request.EntityType,
			TargetID:     uint(request.TargetID),
			ConfirmLabel: request.ConfirmLabel,
			Reason:       request.Reason,
			ExecuteAt:    executeAt,
			Status:       "pending",
			CreatedAt:    time.Now().UTC(),
		}, nil
	}
}

func newCleanupRouteRequest(method string, path string, body []byte) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	routeContext := chi.NewRouteContext()
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	return request
}

func TestCleanupEndpointsRequireAdminAuth(t *testing.T) {
	stubCleanupHooks()
	defer stubCleanupHooks()

	handlers := []http.HandlerFunc{
		AdminCleanupListMarkersHandler,
		AdminCleanupListSchedulesHandler,
		AdminCleanupDeleteMarkerHandler,
		AdminCleanupDeleteScheduleHandler,
		AdminCleanupScheduleJobHandler,
	}

	for _, handler := range handlers {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/admin/cleanup/test", nil)
		handler(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized for %T, got %d", handler, recorder.Code)
		}
	}

	cleanupCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 9}, Role: "user", Username: "tester"}
	}

	for _, handler := range handlers {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/admin/cleanup/test", nil)
		handler(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected forbidden for non-admin %T, got %d", handler, recorder.Code)
		}
	}
}

func TestCleanupListMarkersRejectsInvalidFilters(t *testing.T) {
	stubCleanupHooks()
	defer stubCleanupHooks()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/cleanup/markers?bad_filter=value", nil)
	AdminCleanupListMarkersHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid query controls, got %d", recorder.Code)
	}
}

func TestCleanupDeleteMarkerRequiresExplicitConfirmation(t *testing.T) {
	stubCleanupHooks()
	defer stubCleanupHooks()
	cleanupCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 1}, Role: "admin", Username: "admin"}
	}

	payload := adminCleanupDeleteRequest{ConfirmID: 12, ConfirmLabel: "Marker A"}
	body, _ := json.Marshal(payload)

	recorder := httptest.NewRecorder()
	request := newCleanupRouteRequest(http.MethodDelete, "/admin/cleanup/markers/11", body)
	chi.RouteContext(request.Context()).URLParams.Add("id", "11")

	AdminCleanupDeleteMarkerHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when confirm_id does not match path id, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "confirmation id mismatch") {
		t.Fatalf("expected mismatch error, got %s", recorder.Body.String())
	}
}

func TestCleanupDeleteScheduleRequiresExplicitConfirmation(t *testing.T) {
	stubCleanupHooks()
	defer stubCleanupHooks()
	cleanupCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 1}, Role: "admin", Username: "admin"}
	}

	payload := adminCleanupDeleteRequest{ConfirmID: 5, ConfirmLabel: "Wrong"}
	body, _ := json.Marshal(payload)

	recorder := httptest.NewRecorder()
	request := newCleanupRouteRequest(http.MethodDelete, "/admin/cleanup/schedules/5", body)
	chi.RouteContext(request.Context()).URLParams.Add("id", "5")

	AdminCleanupDeleteScheduleHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when confirm_label mismatches, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "confirmation label mismatch") {
		t.Fatalf("expected label mismatch error, got %s", recorder.Body.String())
	}
}

func TestCleanupScheduleJobValidation(t *testing.T) {
	stubCleanupHooks()
	defer stubCleanupHooks()
	cleanupCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 1}, Role: "admin", Username: "admin"}
	}

	payload := adminCleanupScheduleRequest{
		EntityType:   "marker",
		TargetID:     2,
		ConfirmID:    2,
		ConfirmLabel: "Marker A",
		ExecuteAt:    "invalid",
		Reason:       "later",
	}
	body, _ := json.Marshal(payload)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/cleanup/jobs", bytes.NewReader(body))
	AdminCleanupScheduleJobHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid execute_at, got %d", recorder.Code)
	}
}

func TestCleanupE2ESuccessAndNonAdminRejection(t *testing.T) {
	stubCleanupHooks()
	defer stubCleanupHooks()

	router := chi.NewRouter()
	router.Delete("/admin/cleanup/markers/{id}", AdminCleanupDeleteMarkerHandler)

	cleanupCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 77}, Role: "admin", Username: "admin"}
	}

	okPayload := adminCleanupDeleteRequest{ConfirmID: 7, ConfirmLabel: "Marker A", Reason: "cleanup"}
	okBody, _ := json.Marshal(okPayload)
	okReq := httptest.NewRequest(http.MethodDelete, "/admin/cleanup/markers/7", bytes.NewReader(okBody))
	okRec := httptest.NewRecorder()
	router.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected admin cleanup success status 200, got %d", okRec.Code)
	}

	cleanupCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 88}, Role: "user", Username: "user"}
	}

	denyReq := httptest.NewRequest(http.MethodDelete, "/admin/cleanup/markers/7", bytes.NewReader(okBody))
	denyRec := httptest.NewRecorder()
	router.ServeHTTP(denyRec, denyReq)
	if denyRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin rejection status 403, got %d", denyRec.Code)
	}
}
