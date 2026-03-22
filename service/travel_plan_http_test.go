package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"

	"gorm.io/gorm"

	"github.com/go-chi/chi"
)

func resetTravelPlanHooks() {
	integrationAuthenticateTravelPlanRequestFn = authenticateIntegrationRequest
	integrationAuthenticateTravelPlanRequestJSONFn = authenticateIntegrationRequestJSON
	integrationGetTravelPlanByIDFn = func(planID uint) (*dbmodel.TravelPlan, error) {
		plan := &dbmodel.TravelPlan{}
		plan.ID = planID
		if err := plan.GetWithDailyPlans(database.Connection); err != nil {
			return nil, err
		}
		return plan, nil
	}
	integrationDeleteTravelPlanFn = func(plan *dbmodel.TravelPlan, actor *dbmodel.User) error {
		return nil
	}
	integrationGetTravelPlanDailyByIDFn = func(planID uint, dailyID uint) (*dbmodel.TravelPlanDailyPlan, error) {
		return &dbmodel.TravelPlanDailyPlan{
			BaseModel: dbmodel.BaseModel{ID: dailyID},
		}, nil
	}
	integrationDeleteTravelPlanDailyByIDFn = func(planID uint, dailyID uint) error {
		return nil
	}
}

func setTravelPlanRouteParams(req *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestIntegrationDeleteTravelPlanHandlerScopeDenied(t *testing.T) {
	resetTravelPlanHooks()
	defer resetTravelPlanHooks()

	integrationAuthenticateTravelPlanRequestJSONFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		if requiredScope != constant.APIKeyScopeTravelPlansWrite {
			t.Fatalf("expected scope %s, got %s", constant.APIKeyScopeTravelPlansWrite, requiredScope)
		}
		http.Error(w, "api key scope denied", http.StatusForbidden)
		return nil, false
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/travel-plans/7", nil)
	request = setRouteParam(request, "id", "7")
	IntegrationDeleteTravelPlanHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestIntegrationDeleteTravelPlanHandlerRelationMismatch(t *testing.T) {
	resetTravelPlanHooks()
	defer resetTravelPlanHooks()

	integrationAuthenticateTravelPlanRequestJSONFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}}}, true
	}
	integrationGetTravelPlanByIDFn = func(planID uint) (*dbmodel.TravelPlan, error) {
		return &dbmodel.TravelPlan{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: planID}},
			RelationID: 2,
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/travel-plans/7", nil)
	request = setRouteParam(request, "id", "7")
	IntegrationDeleteTravelPlanHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestIntegrationDeleteTravelPlanHandlerSuccess(t *testing.T) {
	resetTravelPlanHooks()
	defer resetTravelPlanHooks()

	integrationAuthenticateTravelPlanRequestJSONFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		actorUserID := uint(3)
		return &dbmodel.APIKey{
			Relation:    dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 10}},
			ActorUserID: &actorUserID,
			ActorUser:   dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 3}, Username: "api-bot"},
		}, true
	}
	integrationGetTravelPlanByIDFn = func(planID uint) (*dbmodel.TravelPlan, error) {
		return &dbmodel.TravelPlan{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: planID}},
			RelationID: 10,
		}, nil
	}

	deleted := false
	integrationDeleteTravelPlanFn = func(plan *dbmodel.TravelPlan, actor *dbmodel.User) error {
		if plan.ID != 7 {
			t.Fatalf("expected plan id 7, got %d", plan.ID)
		}
		if actor == nil || actor.Username != "api-bot" {
			t.Fatalf("expected actor api-bot")
		}
		deleted = true
		return nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/travel-plans/7", nil)
	request = setRouteParam(request, "id", "7")
	IntegrationDeleteTravelPlanHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if !deleted {
		t.Fatalf("expected integrationDeleteTravelPlanFn to be called")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json payload: %v", err)
	}
	if payload["deleted"] != true {
		t.Fatalf("expected deleted=true, got %+v", payload)
	}
	if payload["entity"] != "travel_plan" {
		t.Fatalf("expected entity=travel_plan, got %+v", payload)
	}
}

func TestIntegrationDeleteTravelPlanDailyPlanHandlerNotFound(t *testing.T) {
	resetTravelPlanHooks()
	defer resetTravelPlanHooks()

	integrationAuthenticateTravelPlanRequestJSONFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
		}, true
	}
	integrationGetTravelPlanByIDFn = func(planID uint) (*dbmodel.TravelPlan, error) {
		return &dbmodel.TravelPlan{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: planID}},
			RelationID: 1,
		}, nil
	}
	integrationGetTravelPlanDailyByIDFn = func(planID uint, dailyID uint) (*dbmodel.TravelPlanDailyPlan, error) {
		return nil, gorm.ErrRecordNotFound
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/travel-plans/7/daily-plans/3", nil)
	request = setTravelPlanRouteParams(request, map[string]string{"id": "7", "daily_id": "3"})
	IntegrationDeleteTravelPlanDailyPlanHandler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

func TestIntegrationDeleteTravelPlanDailyPlanHandlerSuccess(t *testing.T) {
	resetTravelPlanHooks()
	defer resetTravelPlanHooks()

	integrationAuthenticateTravelPlanRequestJSONFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
		}, true
	}
	integrationGetTravelPlanByIDFn = func(planID uint) (*dbmodel.TravelPlan, error) {
		return &dbmodel.TravelPlan{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: planID}},
			RelationID: 1,
		}, nil
	}
	integrationGetTravelPlanDailyByIDFn = func(planID uint, dailyID uint) (*dbmodel.TravelPlanDailyPlan, error) {
		return &dbmodel.TravelPlanDailyPlan{
			BaseModel: dbmodel.BaseModel{ID: dailyID},
		}, nil
	}

	var capturedPlanID uint
	var capturedDailyID uint
	integrationDeleteTravelPlanDailyByIDFn = func(planID uint, dailyID uint) error {
		capturedPlanID = planID
		capturedDailyID = dailyID
		return nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/travel-plans/7/daily-plans/3", nil)
	request = setTravelPlanRouteParams(request, map[string]string{"id": "7", "daily_id": "3"})
	IntegrationDeleteTravelPlanDailyPlanHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if capturedPlanID != 7 || capturedDailyID != 3 {
		t.Fatalf("expected delete call with (7,3), got (%d,%d)", capturedPlanID, capturedDailyID)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json payload: %v", err)
	}
	if payload["deleted"] != true {
		t.Fatalf("expected deleted=true, got %+v", payload)
	}
}

func TestIntegrationDeleteTravelPlanHandlerDeleteError(t *testing.T) {
	resetTravelPlanHooks()
	defer resetTravelPlanHooks()

	integrationAuthenticateTravelPlanRequestJSONFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}},
		}, true
	}
	integrationGetTravelPlanByIDFn = func(planID uint) (*dbmodel.TravelPlan, error) {
		return &dbmodel.TravelPlan{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: planID}},
			RelationID: 1,
		}, nil
	}
	integrationDeleteTravelPlanFn = func(plan *dbmodel.TravelPlan, actor *dbmodel.User) error {
		return errors.New("delete failed")
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/integration/travel-plans/7", nil)
	request = setRouteParam(request, "id", "7")
	IntegrationDeleteTravelPlanHandler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}
