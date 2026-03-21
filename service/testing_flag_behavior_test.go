package service

import (
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"testing"
	"time"
)

func TestCanAccessTestingEntities(t *testing.T) {
	adminRole := "admin"
	normalRole := "user"
	if !canAccessTestingEntities(adminRole) {
		t.Fatalf("expected admin to access testing entities")
	}
	if canAccessTestingEntities(normalRole) {
		t.Fatalf("expected non-admin to be denied testing entities")
	}
}

func TestResolveRequestedTestingFlag(t *testing.T) {
	flagTrue := true
	flagFalse := false

	value, err := resolveRequestedTestingFlag(nil, false)
	if err != nil || value {
		t.Fatalf("expected nil request to default false, got value=%t err=%v", value, err)
	}

	value, err = resolveRequestedTestingFlag(&flagFalse, false)
	if err != nil || value {
		t.Fatalf("expected explicit false to pass for non-admin, got value=%t err=%v", value, err)
	}

	if _, err := resolveRequestedTestingFlag(&flagTrue, false); err == nil {
		t.Fatalf("expected non-admin testing=true to be rejected")
	}

	value, err = resolveRequestedTestingFlag(&flagTrue, true)
	if err != nil || !value {
		t.Fatalf("expected admin testing=true to pass, got value=%t err=%v", value, err)
	}
}

func TestBuildIntegrationScheduleResponseIncludesTestingFlag(t *testing.T) {
	now := time.Now()
	schedule := dbmodel.Schedule{
		Testing: true,
		ObjectBase: dbmodel.ObjectBase{
			BaseModel: dbmodel.BaseModel{
				ID:        8,
				CreatedAt: now,
			},
			UpdatedAt: now,
		},
		Label:       "sample",
		Description: "sample",
		Status:      "",
		SelectedDate: now,
	}

	response := buildIntegrationScheduleResponse(schedule, nil, nil)
	if !response.Testing {
		t.Fatalf("expected testing=true in integration schedule response")
	}
}

func TestBuildIntegrationMarkerResponseUsesTestingArgument(t *testing.T) {
	item := buildIntegrationMarkerResponse(model.Marker{ID: 1, CreatedAt: "2026-03-03T12:30:00Z"}, true)
	if !item.Testing {
		t.Fatalf("expected testing flag in integration marker response")
	}
}
