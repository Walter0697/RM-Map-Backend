package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
)

func resetIntegrationCalendarSyncByDateHooks() {
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	integrationExecuteManualCalendarSyncFn = executeManualCalendarSync
	integrationListRelationSchedulesByDateFn = func(relationID uint, dayStart time.Time, dayEnd time.Time, includeTesting bool) ([]dbmodel.Schedule, error) {
		query := database.Connection.Model(&dbmodel.Schedule{}).Where("relation_id = ?", relationID)
		query = query.Where("selected_date >= ? AND selected_date < ?", dayStart.Format(time.RFC3339), dayEnd.Format(time.RFC3339))
		if !includeTesting {
			query = query.Where("testing = ?", false)
		}
		items := make([]dbmodel.Schedule, 0)
		if err := query.Find(&items).Error; err != nil {
			return nil, err
		}
		return items, nil
	}
	integrationResolveCalendarSyncUserIDFn = func(apiKey *dbmodel.APIKey) (uint, error) {
		if apiKey == nil {
			return 0, fmt.Errorf("api key context missing")
		}
		if apiKey.ActorUser.ID != 0 {
			return apiKey.ActorUser.ID, nil
		}
		if apiKey.ActorUserID != nil && *apiKey.ActorUserID != 0 {
			return *apiKey.ActorUserID, nil
		}
		if apiKey.Relation.UserOneUID != 0 {
			return apiKey.Relation.UserOneUID, nil
		}
		if apiKey.Relation.UserTwoUID != 0 {
			return apiKey.Relation.UserTwoUID, nil
		}
		return 0, fmt.Errorf("unable to resolve calendar sync actor user")
	}
}

func TestIntegrationCalendarGoogleSyncByDateHandlerScopeDenied(t *testing.T) {
	resetIntegrationCalendarSyncByDateHooks()
	defer resetIntegrationCalendarSyncByDateHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		if operation != "integration.calendar.google.sync_by_date" {
			t.Fatalf("unexpected operation %s", operation)
		}
		if requiredScope != constant.APIKeyScopeCalendarSync {
			t.Fatalf("expected required scope %s, got %s", constant.APIKeyScopeCalendarSync, requiredScope)
		}
		http.Error(w, "api key scope denied", http.StatusForbidden)
		return nil, false
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/calendar/google/sync-by-date", strings.NewReader(`{"date":"2026-03-14"}`))
	request.Header.Set("Content-Type", "application/json")

	IntegrationCalendarGoogleSyncByDateHandler(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestIntegrationCalendarGoogleSyncByDateHandlerSuccessWithPartialFailures(t *testing.T) {
	resetIntegrationCalendarSyncByDateHooks()
	defer resetIntegrationCalendarSyncByDateHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{
			Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 90}},
			ActorUser: dbmodel.User{
				BaseModel: dbmodel.BaseModel{ID: 7},
				Role:      "admin",
			},
		}, true
	}
	integrationResolveCalendarSyncUserIDFn = func(apiKey *dbmodel.APIKey) (uint, error) {
		return 7, nil
	}
	integrationListRelationSchedulesByDateFn = func(relationID uint, dayStart time.Time, dayEnd time.Time, includeTesting bool) ([]dbmodel.Schedule, error) {
		if relationID != 90 {
			t.Fatalf("unexpected relation id %d", relationID)
		}
		return []dbmodel.Schedule{
			{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1}}},
			{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 2}}},
		}, nil
	}
	integrationExecuteManualCalendarSyncFn = func(ctx context.Context, userID uint, scheduleID uint, providerKey string, requestedAction string) (*calendarManualSyncResult, error) {
		if userID != 7 {
			t.Fatalf("unexpected sync user id %d", userID)
		}
		if providerKey != CalendarProviderGoogle {
			t.Fatalf("unexpected provider %s", providerKey)
		}
		if scheduleID == 2 {
			return nil, fmt.Errorf("provider timeout")
		}
		return &calendarManualSyncResult{
			SyncStatus: dbmodel.CalendarSyncStatusSynced,
			Action:     dbmodel.CalendarSyncJobActionCreate,
			Provider:   CalendarProviderGoogle,
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/calendar/google/sync-by-date", strings.NewReader(`{"date":"2026-03-14"}`))
	request.Header.Set("Content-Type", "application/json")

	IntegrationCalendarGoogleSyncByDateHandler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload integrationCalendarGoogleSyncByDateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if payload.Total != 2 || payload.Synced != 1 || payload.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", payload)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload.Items))
	}
}
