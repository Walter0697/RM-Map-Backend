package service

import (
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

	"gorm.io/gorm"
)

func resetIntegrationCreateScheduleHooks() {
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	integrationGetScheduleMarkerForCreateFn = func(markerID uint) (*dbmodel.Marker, error) {
		marker := &dbmodel.Marker{}
		marker.ID = markerID
		if err := marker.GetById(database.Connection); err != nil {
			return nil, err
		}
		return marker, nil
	}
	integrationBeginCreateScheduleTxFn = func() (*gorm.DB, error) {
		tx := database.Connection.Begin()
		if tx.Error != nil {
			return nil, tx.Error
		}
		return tx, nil
	}
	integrationCommitCreateScheduleTxFn = func(tx *gorm.DB) error {
		return tx.Commit().Error
	}
	integrationRollbackCreateScheduleTxFn = func(tx *gorm.DB) {
		if tx == nil {
			return
		}
		tx.Rollback()
	}
	integrationCreateScheduleFn = CreateSchedule
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error {
		return schedule.Update(tx)
	}
	integrationLinkScheduleToTravelPlanItemFn = linkScheduleToTravelPlanItem
}

func TestIntegrationCreateScheduleHandlerLinksTravelPlanItemSuccess(t *testing.T) {
	resetIntegrationCreateScheduleHooks()
	defer resetIntegrationCreateScheduleHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		if operation != "integration.schedules.create" {
			t.Fatalf("unexpected operation: %s", operation)
		}
		if requiredScope != constant.APIKeyScopeSchedulesWrite {
			t.Fatalf("unexpected scope: %s", requiredScope)
		}
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 99}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 7},
				Role:      "admin",
			},
		}, true
	}
	integrationGetScheduleMarkerForCreateFn = func(markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 99,
		}, nil
	}
	integrationBeginCreateScheduleTxFn = func() (*gorm.DB, error) { return &gorm.DB{}, nil }
	integrationCreateScheduleFn = func(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation, testing bool) (*dbmodel.Schedule, error) {
		selectedAt := time.Date(2026, 3, 21, 12, 30, 0, 0, time.UTC)
		return &dbmodel.Schedule{
			ObjectBase:   dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 501}},
			Label:        input.Label,
			Description:  input.Description,
			SelectedDate: selectedAt,
			RelationId:   relation.ID,
			Testing:      testing,
		}, nil
	}
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error { return nil }

	linkCalled := false
	integrationLinkScheduleToTravelPlanItemFn = func(tx *gorm.DB, relationID uint, context integrationTravelPlanItemLinkContext, requestItem *integrationScheduleItemInput, scheduleID uint) (*integrationTravelPlanItemLinkResponse, error) {
		linkCalled = true
		if relationID != 99 || context.TravelPlanID != 123 || context.ItemID != 456 || scheduleID != 501 {
			t.Fatalf("unexpected link context: relation=%d context=%+v schedule=%d", relationID, context, scheduleID)
		}
		return &integrationTravelPlanItemLinkResponse{TravelPlanID: 123, ItemID: 456, ScheduleID: 501}, nil
	}

	committed := false
	integrationCommitCreateScheduleTxFn = func(tx *gorm.DB) error {
		committed = true
		return nil
	}
	integrationRollbackCreateScheduleTxFn = func(tx *gorm.DB) {
		t.Fatalf("did not expect rollback")
	}

	body := `{"label":"test","description":"desc","selected_time":"2026-03-21 12:30:00+00","marker_id":42,"travel_plan_id":123,"item_id":456}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/schedules", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationCreateScheduleHandler(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !linkCalled {
		t.Fatalf("expected travel plan link to be called")
	}
	if !committed {
		t.Fatalf("expected transaction commit")
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	linkRaw, ok := payload["travel_plan_item_link"].(map[string]any)
	if !ok {
		t.Fatalf("expected travel_plan_item_link in response: %v", payload)
	}
	if linkRaw["travel_plan_id"] != float64(123) || linkRaw["item_id"] != float64(456) || linkRaw["schedule_id"] != float64(501) {
		t.Fatalf("unexpected travel_plan_item_link payload: %v", linkRaw)
	}
}

func TestIntegrationCreateScheduleHandlerAllowsRetryPayloadWithMatchingItemScheduleID(t *testing.T) {
	resetIntegrationCreateScheduleHooks()
	defer resetIntegrationCreateScheduleHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 77}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 8},
				Role:      "admin",
			},
		}, true
	}
	integrationGetScheduleMarkerForCreateFn = func(markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}}, RelationId: 77}, nil
	}
	integrationBeginCreateScheduleTxFn = func() (*gorm.DB, error) { return &gorm.DB{}, nil }
	integrationCreateScheduleFn = func(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation, testing bool) (*dbmodel.Schedule, error) {
		return &dbmodel.Schedule{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 777}}, RelationId: 77}, nil
	}
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error { return nil }

	integrationLinkScheduleToTravelPlanItemFn = func(tx *gorm.DB, relationID uint, context integrationTravelPlanItemLinkContext, requestItem *integrationScheduleItemInput, scheduleID uint) (*integrationTravelPlanItemLinkResponse, error) {
		if requestItem == nil || requestItem.ScheduleID == nil || *requestItem.ScheduleID != 777 {
			t.Fatalf("expected retry payload to carry matching item.schedule_id, got %+v", requestItem)
		}
		if scheduleID != 777 {
			t.Fatalf("expected schedule id 777, got %d", scheduleID)
		}
		return &integrationTravelPlanItemLinkResponse{TravelPlanID: 88, ItemID: 99, ScheduleID: 777}, nil
	}
	integrationCommitCreateScheduleTxFn = func(tx *gorm.DB) error { return nil }
	integrationRollbackCreateScheduleTxFn = func(tx *gorm.DB) {
		t.Fatalf("did not expect rollback")
	}

	body := `{"label":"retry","description":"desc","selected_time":"2026-03-21 12:30:00+00","marker_id":42,"travel_plan_id":88,"item_id":99,"item":{"schedule_id":777}}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/schedules", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationCreateScheduleHandler(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestIntegrationCreateScheduleHandlerReturnsExplicitLinkFailure(t *testing.T) {
	resetIntegrationCreateScheduleHooks()
	defer resetIntegrationCreateScheduleHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 66}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 9},
				Role:      "admin",
			},
		}, true
	}
	integrationGetScheduleMarkerForCreateFn = func(markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}}, RelationId: 66}, nil
	}
	integrationBeginCreateScheduleTxFn = func() (*gorm.DB, error) { return &gorm.DB{}, nil }
	integrationCreateScheduleFn = func(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation, testing bool) (*dbmodel.Schedule, error) {
		return &dbmodel.Schedule{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 909}}, RelationId: 66}, nil
	}
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error { return nil }
	integrationLinkScheduleToTravelPlanItemFn = func(tx *gorm.DB, relationID uint, context integrationTravelPlanItemLinkContext, requestItem *integrationScheduleItemInput, scheduleID uint) (*integrationTravelPlanItemLinkResponse, error) {
		return nil, &integrationScheduleLinkError{
			StatusCode: http.StatusInternalServerError,
			Code:       "travel_plan_item_link_failed",
			Message:    "failed to persist travel plan item schedule link",
		}
	}
	integrationCommitCreateScheduleTxFn = func(tx *gorm.DB) error {
		t.Fatalf("did not expect commit")
		return nil
	}
	rolledBack := false
	integrationRollbackCreateScheduleTxFn = func(tx *gorm.DB) {
		rolledBack = true
	}

	body := `{"label":"test","description":"desc","selected_time":"2026-03-21 12:30:00+00","marker_id":42,"travel_plan_id":1,"item_id":2}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/schedules", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationCreateScheduleHandler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !rolledBack {
		t.Fatalf("expected rollback on link failure")
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if payload["code"] != "travel_plan_item_link_failed" {
		t.Fatalf("expected explicit error code, got %v", payload)
	}
}
