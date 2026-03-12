package service

import (
	"mapmarker/backend/database/dbmodel"
	"testing"
	"time"
)

func TestBuildExportSnapshotWebsiteFallbackAndEstimatedTimeFallback(t *testing.T) {
	now := time.Date(2026, 3, 10, 1, 2, 3, 0, time.UTC)
	input := BuildExportSnapshotInput{
		RelationID: 88,
		UserID:     99,
		Timezone:   "Asia/Hong_Kong",
		Now:        now,
		Markers: []dbmodel.Marker{
			{
				ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 2}},
				Label:      "Fallback Marker",
				Link:       "https://example.com/fallback",
			},
			{
				ObjectBase:   dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1}},
				Label:        "Provider Marker",
				EstimateTime: "30m",
				RestaurantInfo: &dbmodel.Restaurant{
					Source:   "yelp",
					SourceId: "abc123",
					Website:  "https://yelp.com/biz/abc123",
				},
			},
		},
		Schedules: []dbmodel.Schedule{
			{
				ObjectBase:     dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 5}},
				Label:          "Dinner",
				SelectedDate:   now,
				SelectedMarker: &dbmodel.Marker{EstimateTime: "15m"},
			},
			{
				ObjectBase:   dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 6}},
				Label:        "No marker",
				SelectedDate: now.Add(time.Hour),
			},
		},
	}

	snapshot, err := BuildExportSnapshot(input)
	if err != nil {
		t.Fatalf("BuildExportSnapshot returned error: %v", err)
	}

	if snapshot.SchemaVersion != ExportSnapshotSchemaVersion {
		t.Fatalf("expected schema version %s, got %s", ExportSnapshotSchemaVersion, snapshot.SchemaVersion)
	}
	if snapshot.GeneratedAt != now {
		t.Fatalf("expected generated_at %v, got %v", now, snapshot.GeneratedAt)
	}
	if snapshot.Timezone != "Asia/Hong_Kong" {
		t.Fatalf("expected timezone Asia/Hong_Kong, got %s", snapshot.Timezone)
	}

	if len(snapshot.Markers) != 2 {
		t.Fatalf("expected 2 markers, got %d", len(snapshot.Markers))
	}
	if snapshot.Markers[0].ID != 1 || snapshot.Markers[1].ID != 2 {
		t.Fatalf("expected markers sorted by id, got %d then %d", snapshot.Markers[0].ID, snapshot.Markers[1].ID)
	}

	firstWebsite := snapshot.Markers[0].Website
	if firstWebsite.Provider != "yelp" || firstWebsite.ProviderID != "abc123" || firstWebsite.Status != offlineExportWebsiteStatusAvailable {
		t.Fatalf("unexpected provider website projection: %+v", firstWebsite)
	}
	secondWebsite := snapshot.Markers[1].Website
	if secondWebsite.Status != offlineExportWebsiteStatusFallback || secondWebsite.URL != "https://example.com/fallback" {
		t.Fatalf("expected fallback website for marker 2, got %+v", secondWebsite)
	}

	if snapshot.Markers[0].EstimateTime.Missing {
		t.Fatalf("expected marker estimated time present")
	}
	if snapshot.Markers[1].EstimateTime.Display != "N/A" || !snapshot.Markers[1].EstimateTime.Missing {
		t.Fatalf("expected marker 2 estimated time fallback, got %+v", snapshot.Markers[1].EstimateTime)
	}

	if len(snapshot.Schedules) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(snapshot.Schedules))
	}
	if snapshot.Schedules[0].EstimateTime.Display != "15m" {
		t.Fatalf("expected schedule estimate to project marker estimate, got %+v", snapshot.Schedules[0].EstimateTime)
	}
	if !snapshot.Schedules[1].EstimateTime.Missing {
		t.Fatalf("expected schedule estimate fallback for missing marker")
	}
}

func TestBuildExportSnapshotTimezoneValidation(t *testing.T) {
	_, err := BuildExportSnapshot(BuildExportSnapshotInput{Timezone: "NoSuch/TZ"})
	if err == nil {
		t.Fatalf("expected timezone validation error")
	}
}
