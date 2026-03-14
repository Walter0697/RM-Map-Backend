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
	integrationAuthenticateNearbyRequestFn = authenticateIntegrationRequestJSON
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
	integrationFindNearbyMarkersFn = findNearbyMarkersByDistance
	integrationResolveRestaurantByProviderFn = GetOrCreateRestaurantByProvider
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

	item := buildIntegrationMarkerResponse(model.Marker{ID: 99, CreatedAt: "2026-03-03T12:30:00Z"}, false)
	if item.IntegrationSource != "api_key" {
		t.Fatalf("unexpected integration source: %s", item.IntegrationSource)
	}
	if item.CreatedAgo != "1 hour ago" {
		t.Fatalf("expected 1 hour ago, got %s", item.CreatedAgo)
	}
	if !item.EditableWithSameAPIKey || !item.RemovableWithSameAPIKey {
		t.Fatalf("expected editable/removable flags true")
	}
	if item.WebsiteIntegration == nil || item.WebsiteIntegration.FetchStatus != "no_integration" {
		t.Fatalf("expected no_integration website payload, got %+v", item.WebsiteIntegration)
	}
}

func TestBuildIntegrationMarkerResponseWebsitePayload(t *testing.T) {
	item := buildIntegrationMarkerResponse(model.Marker{
		ID:        10,
		CreatedAt: "2026-03-03T12:30:00Z",
		Restaurant: &model.Restaurant{
			Source:   "openrice",
			SourceID: "abc123",
			Name:     "Demo Place",
			Rating:   stringPtr(`{"like":"80","average":"10","dislike":"10"}`),
			Website:  stringPtr("https://s.openrice.com/abc123"),
		},
	}, false)
	if item.WebsiteIntegration == nil {
		t.Fatalf("expected website integration payload")
	}
	if item.WebsiteIntegration.Provider != "openrice" || item.WebsiteIntegration.ExternalID != "abc123" {
		t.Fatalf("unexpected provider payload: %+v", item.WebsiteIntegration)
	}
	if item.WebsiteIntegration.Rating == nil || !strings.Contains(*item.WebsiteIntegration.Rating, "like 80") {
		t.Fatalf("expected normalized rating, got %+v", item.WebsiteIntegration.Rating)
	}
}

func TestIntegrationNearbySearchMarkersHandlerValidationError(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/integration/markers/nearby?latitude=91&longitude=114.17&radius=200", nil)
	IntegrationNearbySearchMarkersHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}

	response := integrationErrorResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected json error response: %v", err)
	}
	if response.Code != "invalid_nearby_search_input" {
		t.Fatalf("expected invalid_nearby_search_input, got %s", response.Code)
	}
}

func TestIntegrationNearbySearchMarkersHandlerScopeDenied(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateNearbyRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		writeIntegrationError(w, http.StatusForbidden, "api_key_scope_denied", "api key scope denied")
		return nil, false
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/integration/markers/nearby?latitude=22.3&longitude=114.17&radius=200", nil)
	IntegrationNearbySearchMarkersHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}

	response := integrationErrorResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected json error response: %v", err)
	}
	if response.Code != "api_key_scope_denied" {
		t.Fatalf("expected api_key_scope_denied, got %s", response.Code)
	}
}

func TestIntegrationNearbySearchMarkersHandlerSuccessDistanceOrdering(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateNearbyRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
		}, true
	}
	integrationFindNearbyMarkersFn = func(relation dbmodel.UserRelation, params integrationNearbySearchQuery, includeTesting bool) ([]integrationNearbyMarkerRow, error) {
		return []integrationNearbyMarkerRow{
			{
				Marker: dbmodel.Marker{
					ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1, CreatedAt: time.Now().Add(-2 * time.Hour)}},
					Label:      "Near Marker",
					Latitude:   22.3001,
					Longitude:  114.1701,
				},
				DistanceMeters: 12.4,
			},
			{
				Marker: dbmodel.Marker{
					ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 2, CreatedAt: time.Now().Add(-2 * time.Hour)}},
					Label:      "Far Marker",
					Latitude:   22.301,
					Longitude:  114.171,
				},
				DistanceMeters: 88.1,
			},
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/integration/markers/nearby?latitude=22.3&longitude=114.17&radius=300&limit=10", nil)
	IntegrationNearbySearchMarkersHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var payload struct {
		Items []struct {
			ID             int     `json:"id"`
			Label          string  `json:"label"`
			DistanceMeters float64 `json:"distance_meters"`
		} `json:"items"`
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if payload.Limit != 10 {
		t.Fatalf("expected limit=10, got %d", payload.Limit)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload.Items))
	}
	if payload.Items[0].DistanceMeters >= payload.Items[1].DistanceMeters {
		t.Fatalf("expected distance ordering asc, got %+v", payload.Items)
	}
}

func TestIntegrationNearbySearchMarkersHandlerEmptyResult(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateNearbyRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
		}, true
	}
	integrationFindNearbyMarkersFn = func(relation dbmodel.UserRelation, params integrationNearbySearchQuery, includeTesting bool) ([]integrationNearbyMarkerRow, error) {
		return []integrationNearbyMarkerRow{}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/integration/markers/nearby?latitude=22.3&longitude=114.17&radius=50", nil)
	IntegrationNearbySearchMarkersHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var payload struct {
		Items []integrationNearbyMarkerResponse `json:"items"`
		Total int                               `json:"total"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if payload.Total != 0 || len(payload.Items) != 0 {
		t.Fatalf("expected empty list response, got %+v", payload)
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

func TestIntegrationCreateMarkerHandlerRejectsConflictingWebsiteFields(t *testing.T) {
	resetIntegrationMarkerHooks()
	defer resetIntegrationMarkerHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation:  dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
			ActorUser: dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 9}, Username: "api-bot"},
		}, true
	}

	body := `{"label":"site","address":"test","type":"food","latitude":22.3,"longitude":114.17,"restaurant_id":12,"website_provider":"yelp","website_provider_id":"abc-id"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/markers", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationCreateMarkerHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	response := integrationErrorResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if response.Code != "invalid_website_integration" {
		t.Fatalf("unexpected code %s", response.Code)
	}
}

func stringPtr(value string) *string {
	return &value
}
