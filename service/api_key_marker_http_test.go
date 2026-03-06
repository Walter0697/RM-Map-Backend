package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"

	"github.com/go-chi/chi"
)

func setRouteParam(req *http.Request, key string, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func resetIntegrationMarkerHooks() {
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	integrationGetMarkerByIDFn = func(id uint) (*dbmodel.Marker, error) {
		marker := &dbmodel.Marker{}
		marker.ID = id
		if err := marker.GetById(database.Connection); err != nil {
			return nil, err
		}
		return marker, nil
	}
	integrationUpdateMarkerModelFn = func(marker *dbmodel.Marker) error {
		return marker.Update(database.Connection)
	}
	integrationNowFn = time.Now
}

func TestIntegrationDeleteMarkerHandlerScopeDenied(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		if requiredScope != constant.APIKeyScopeMarkersWrite {
			t.Fatalf("expected scope %s, got %s", constant.APIKeyScopeMarkersWrite, requiredScope)
		}
		http.Error(w, "api key scope denied", http.StatusForbidden)
		return nil, false
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/markers/7", nil)
	request = setRouteParam(request, "id", "7")
	IntegrationDeleteMarkerHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestIntegrationDeleteMarkerHandlerRelationMismatch(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}}}, true
	}
	integrationGetMarkerByIDFn = func(id uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: id}}, RelationId: 2}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/markers/7", nil)
	request = setRouteParam(request, "id", "7")
	IntegrationDeleteMarkerHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestIntegrationDeleteMarkerHandlerSuccess(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation:  dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
			ActorUser: dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 9}, Username: "api-bot"},
		}, true
	}
	integrationGetMarkerByIDFn = func(id uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: id}}, RelationId: 1}, nil
	}
	integrationUpdateMarkerModelFn = func(marker *dbmodel.Marker) error {
		if marker.Status != constant.Cancelled {
			t.Fatalf("expected cancelled status, got %s", marker.Status)
		}
		if marker.UpdatedBy == nil || marker.UpdatedBy.Username != "api-bot" {
			t.Fatalf("expected UpdatedBy to be api-bot")
		}
		return nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/markers/7", nil)
	request = setRouteParam(request, "id", "7")
	IntegrationDeleteMarkerHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	payload := map[string]interface{}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if payload["status"] != "cancelled" {
		t.Fatalf("expected cancelled status, got %+v", payload)
	}
}

func TestBuildIntegrationMarkerResponseAddsIndicator(t *testing.T) {
	fixedNow := time.Date(2026, time.March, 3, 14, 0, 0, 0, time.UTC)
	integrationNowFn = func() time.Time { return fixedNow }
	defer func() { integrationNowFn = time.Now }()

	item := buildIntegrationMarkerResponse(model.Marker{ID: 99, CreatedAt: "2026-03-03T12:30:00Z"})
	if item.IntegrationSource != "api_key" {
		t.Fatalf("unexpected integration source: %s", item.IntegrationSource)
	}
	if item.CreatedAgo != "1 hour ago" {
		t.Fatalf("expected 1 hour ago, got %s", item.CreatedAgo)
	}
	if !item.EditableWithSameAPIKey || !item.RemovableWithSameAPIKey {
		t.Fatalf("expected editable/removable flags true")
	}
}

func TestIntegrationCreateMarkerHandlerRejectsZeroCoordinates(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation:  dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
			ActorUser: dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 9}, Username: "api-bot"},
		}, true
	}

	body := `{"label":"invalid-coords","address":"test","type":"workshop"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/markers", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationCreateMarkerHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(strings.ToLower(recorder.Body.String()), "invalid coordinates") {
		t.Fatalf("expected invalid coordinates error, got %s", recorder.Body.String())
	}
}
