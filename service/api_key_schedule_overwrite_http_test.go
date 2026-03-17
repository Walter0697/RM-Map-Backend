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

func resetIntegrationScheduleOverwriteHooks() {
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	integrationBeginScheduleOverwriteTxFn = func() (*gorm.DB, error) {
		tx := database.Connection.Begin()
		if tx.Error != nil {
			return nil, tx.Error
		}
		return tx, nil
	}
	integrationListSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) ([]dbmodel.Schedule, error) {
		items := make([]dbmodel.Schedule, 0)
		err := tx.Preload("SelectedMarker").
			Where("relation_id = ?", relationID).
			Where("selected_date >= ? AND selected_date < ?", start.Format(time.RFC3339), end.Format(time.RFC3339)).
			Find(&items).Error
		return items, err
	}
	integrationDeleteSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) (int64, error) {
		result := tx.Where("relation_id = ?", relationID).
			Where("selected_date >= ? AND selected_date < ?", start.Format(time.RFC3339), end.Format(time.RFC3339)).
			Delete(&dbmodel.Schedule{})
		return result.RowsAffected, result.Error
	}
	integrationGetScheduleMarkerByIDFn = func(tx *gorm.DB, markerID uint) (*dbmodel.Marker, error) {
		marker := &dbmodel.Marker{}
		marker.ID = markerID
		if err := marker.GetById(tx); err != nil {
			return nil, err
		}
		return marker, nil
	}
	integrationCreateScheduleFn = CreateSchedule
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error {
		return schedule.Update(tx)
	}
	integrationResetMarkerStatusForOverwriteFn = func(tx *gorm.DB, marker *dbmodel.Marker, actor dbmodel.User) error {
		if marker == nil {
			return nil
		}
		marker.Status = ""
		marker.UpdatedBy = &actor
		return marker.Update(tx)
	}
	integrationCommitScheduleOverwriteTxFn = func(tx *gorm.DB) error {
		return tx.Commit().Error
	}
	integrationRollbackScheduleOverwriteTxFn = func(tx *gorm.DB) {
		if tx == nil {
			return
		}
		tx.Rollback()
	}
}

func TestIntegrationOverwriteSchedulesByDateHandlerUnauthorized(t *testing.T) {
	resetIntegrationScheduleOverwriteHooks()
	defer resetIntegrationScheduleOverwriteHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		if operation != "integration.schedules.overwrite_by_date" {
			t.Fatalf("unexpected operation: %s", operation)
		}
		if requiredScope != constant.APIKeyScopeSchedulesWrite {
			t.Fatalf("expected schedule write scope, got %s", requiredScope)
		}
		http.Error(w, "missing api key", http.StatusUnauthorized)
		return nil, false
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/integration/schedules/overwrite-by-date", strings.NewReader(`{"date":"2026-03-14","items":[]}`))
	request.Header.Set("Content-Type", "application/json")
	IntegrationOverwriteSchedulesByDateHandler(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestIntegrationOverwriteSchedulesByDateHandlerScopeDenied(t *testing.T) {
	resetIntegrationScheduleOverwriteHooks()
	defer resetIntegrationScheduleOverwriteHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		http.Error(w, "api key scope denied", http.StatusForbidden)
		return nil, false
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/integration/schedules/overwrite-by-date", strings.NewReader(`{"date":"2026-03-14","items":[]}`))
	request.Header.Set("Content-Type", "application/json")
	IntegrationOverwriteSchedulesByDateHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestIntegrationOverwriteSchedulesByDateHandlerSuccessRemovesStaleEntries(t *testing.T) {
	resetIntegrationScheduleOverwriteHooks()
	defer resetIntegrationScheduleOverwriteHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 99}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 7},
				Username:  "api-bot",
			},
		}, true
	}
	integrationBeginScheduleOverwriteTxFn = func() (*gorm.DB, error) {
		return &gorm.DB{}, nil
	}

	resetMarkerCalls := 0
	integrationListSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) ([]dbmodel.Schedule, error) {
		oldMarker := &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 41}}, RelationId: relationID}
		return []dbmodel.Schedule{
			{
				ObjectBase:     dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 300}},
				RelationId:     relationID,
				SelectedMarker: oldMarker,
			},
		}, nil
	}
	integrationResetMarkerStatusForOverwriteFn = func(tx *gorm.DB, marker *dbmodel.Marker, actor dbmodel.User) error {
		resetMarkerCalls++
		if marker == nil || marker.ID != 41 {
			t.Fatalf("expected stale marker 41 reset")
		}
		return nil
	}

	deletedCalls := 0
	integrationDeleteSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) (int64, error) {
		deletedCalls++
		return 1, nil
	}
	integrationGetScheduleMarkerByIDFn = func(tx *gorm.DB, markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 99,
		}, nil
	}
	integrationCreateScheduleFn = func(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation, testing bool) (*dbmodel.Schedule, error) {
		selectedAt, err := time.Parse("2006-01-02 15:04:05+00", input.SelectedTime)
		if err != nil {
			return nil, err
		}
		return &dbmodel.Schedule{
			ObjectBase:     dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 501}},
			Label:          input.Label,
			Description:    input.Description,
			Testing:        testing,
			MarkerId:       &marker.ID,
			SelectedDate:   selectedAt,
			RelationId:     relation.ID,
			SelectedMarker: &marker,
		}, nil
	}
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error {
		return nil
	}
	committed := false
	integrationCommitScheduleOverwriteTxFn = func(tx *gorm.DB) error {
		committed = true
		return nil
	}
	integrationRollbackScheduleOverwriteTxFn = func(tx *gorm.DB) {}

	body := `{
		"date":"2026-03-14",
		"items":[
			{"label":"new schedule","description":"replace","selected_time":"2026-03-14 12:30:00+00","marker_id":42}
		]
	}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/integration/schedules/overwrite-by-date", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationOverwriteSchedulesByDateHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if resetMarkerCalls != 1 {
		t.Fatalf("expected stale marker reset once, got %d", resetMarkerCalls)
	}
	if deletedCalls != 1 {
		t.Fatalf("expected delete call once, got %d", deletedCalls)
	}
	if !committed {
		t.Fatalf("expected overwrite transaction commit")
	}

	var payload integrationOverwriteScheduleByDateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if payload.Date != "2026-03-14" {
		t.Fatalf("expected date 2026-03-14, got %s", payload.Date)
	}
	if payload.DeletedCount != 1 || payload.CreatedCount != 1 || len(payload.Items) != 1 {
		t.Fatalf("unexpected overwrite payload: %+v", payload)
	}
}

func TestIntegrationOverwriteSchedulesByDateHandlerForcesTestingScheduleLabel(t *testing.T) {
	resetIntegrationScheduleOverwriteHooks()
	defer resetIntegrationScheduleOverwriteHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Testing:  true,
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 99}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 7},
				Username:  "api-bot",
				Role:      "admin",
			},
		}, true
	}
	integrationBeginScheduleOverwriteTxFn = func() (*gorm.DB, error) {
		return &gorm.DB{}, nil
	}
	integrationListSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) ([]dbmodel.Schedule, error) {
		return []dbmodel.Schedule{}, nil
	}
	integrationDeleteSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) (int64, error) {
		return 0, nil
	}
	integrationGetScheduleMarkerByIDFn = func(tx *gorm.DB, markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 99,
		}, nil
	}
	integrationCreateScheduleFn = func(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation, testing bool) (*dbmodel.Schedule, error) {
		if input.Label != "testing schedule" {
			t.Fatalf("expected label to be forced to 'testing schedule', got %q", input.Label)
		}
		if !testing {
			t.Fatalf("expected testing schedule flag to be true for testing api key")
		}
		selectedAt, err := time.Parse("2006-01-02 15:04:05+00", input.SelectedTime)
		if err != nil {
			return nil, err
		}
		return &dbmodel.Schedule{
			ObjectBase:   dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 777}},
			Label:        input.Label,
			Description:  input.Description,
			Testing:      testing,
			SelectedDate: selectedAt,
			RelationId:   relation.ID,
		}, nil
	}
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error { return nil }
	integrationCommitScheduleOverwriteTxFn = func(tx *gorm.DB) error { return nil }
	integrationRollbackScheduleOverwriteTxFn = func(tx *gorm.DB) {}

	body := `{
		"date":"2026-03-14",
		"items":[
			{"label":"normal-label","description":"replace","selected_time":"2026-03-14 12:30:00+00","marker_id":42}
		]
	}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/integration/schedules/overwrite-by-date", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationOverwriteSchedulesByDateHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload integrationOverwriteScheduleByDateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one schedule in payload, got %d", len(payload.Items))
	}
	if payload.Items[0].Label != "testing schedule" {
		t.Fatalf("expected output label testing schedule, got %q", payload.Items[0].Label)
	}
	if !payload.Items[0].Testing {
		t.Fatalf("expected output testing flag true")
	}
}

func TestIntegrationOverwriteSchedulesByDateHandlerAcceptsClockOnlyTime(t *testing.T) {
	resetIntegrationScheduleOverwriteHooks()
	defer resetIntegrationScheduleOverwriteHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 99}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 7},
				Username:  "api-bot",
			},
		}, true
	}
	integrationBeginScheduleOverwriteTxFn = func() (*gorm.DB, error) {
		return &gorm.DB{}, nil
	}
	integrationListSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) ([]dbmodel.Schedule, error) {
		return []dbmodel.Schedule{}, nil
	}
	integrationDeleteSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) (int64, error) {
		return 0, nil
	}
	integrationGetScheduleMarkerByIDFn = func(tx *gorm.DB, markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 99,
		}, nil
	}
	integrationCreateScheduleFn = func(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation, testing bool) (*dbmodel.Schedule, error) {
		if input.SelectedTime != "2026-03-14 08:00:00" {
			t.Fatalf("expected selected_time normalized with target date, got %q", input.SelectedTime)
		}
		selectedAt, err := time.Parse("2006-01-02 15:04:05", input.SelectedTime)
		if err != nil {
			return nil, err
		}
		return &dbmodel.Schedule{
			ObjectBase:   dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 778}},
			Label:        input.Label,
			Description:  input.Description,
			Testing:      testing,
			SelectedDate: selectedAt,
			RelationId:   relation.ID,
			SelectedMarker: &marker,
		}, nil
	}
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error { return nil }
	integrationCommitScheduleOverwriteTxFn = func(tx *gorm.DB) error { return nil }
	integrationRollbackScheduleOverwriteTxFn = func(tx *gorm.DB) {}

	body := `{
		"date":"2026-03-14",
		"items":[
			{"label":"clock-only","description":"replace","selected_time":"08:00","marker_id":42}
		]
	}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/integration/schedules/overwrite-by-date", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationOverwriteSchedulesByDateHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestIntegrationOverwriteSchedulesByDateHandlerAcceptsLocalDateTimeWithoutSeconds(t *testing.T) {
	resetIntegrationScheduleOverwriteHooks()
	defer resetIntegrationScheduleOverwriteHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 99}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 7},
				Username:  "api-bot",
			},
		}, true
	}
	integrationBeginScheduleOverwriteTxFn = func() (*gorm.DB, error) {
		return &gorm.DB{}, nil
	}
	integrationListSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) ([]dbmodel.Schedule, error) {
		return []dbmodel.Schedule{}, nil
	}
	integrationDeleteSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) (int64, error) {
		return 0, nil
	}
	integrationGetScheduleMarkerByIDFn = func(tx *gorm.DB, markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 99,
		}, nil
	}
	integrationCreateScheduleFn = func(tx *gorm.DB, input model.NewSchedule, marker dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation, testing bool) (*dbmodel.Schedule, error) {
		if input.SelectedTime != "2026-03-14 08:00" {
			t.Fatalf("expected local datetime to pass through, got %q", input.SelectedTime)
		}
		selectedAt, err := time.Parse("2006-01-02 15:04", input.SelectedTime)
		if err != nil {
			return nil, err
		}
		return &dbmodel.Schedule{
			ObjectBase:   dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 779}},
			Label:        input.Label,
			Description:  input.Description,
			Testing:      testing,
			SelectedDate: selectedAt,
			RelationId:   relation.ID,
			SelectedMarker: &marker,
		}, nil
	}
	integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error { return nil }
	integrationCommitScheduleOverwriteTxFn = func(tx *gorm.DB) error { return nil }
	integrationRollbackScheduleOverwriteTxFn = func(tx *gorm.DB) {}

	body := `{
		"date":"2026-03-14",
		"items":[
			{"label":"local-datetime","description":"replace","selected_time":"2026-03-14 08:00","marker_id":42}
		]
	}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/integration/schedules/overwrite-by-date", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	IntegrationOverwriteSchedulesByDateHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
