package service

import (
	"testing"

	"mapmarker/backend/database/dbmodel"
)

func TestApplyRoutePreviewToScheduleSetsFields(t *testing.T) {
	schedule := &dbmodel.Schedule{}
	imageRef := "https://example.com/route.png"
	distance := 1234
	walk := 900
	bus := 1200

	warnings, err := applyRoutePreviewToSchedule(schedule, &integrationScheduleRoutePreviewRequest{
		ImageRef:       &imageRef,
		DistanceMeters: &distance,
		ETA: &integrationScheduleRouteETARequest{
			WalkingSeconds: &walk,
			BusSeconds:     &bus,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
	if schedule.RouteImageRef == nil || *schedule.RouteImageRef != imageRef {
		t.Fatalf("expected route image ref to be set")
	}
	if schedule.RouteDistanceMeters == nil || *schedule.RouteDistanceMeters != distance {
		t.Fatalf("expected route distance to be set")
	}
	if schedule.RouteETAWalkingSeconds == nil || *schedule.RouteETAWalkingSeconds != walk {
		t.Fatalf("expected walking eta to be set")
	}
	if schedule.RouteETABusSeconds == nil || *schedule.RouteETABusSeconds != bus {
		t.Fatalf("expected bus eta to be set")
	}
}

func TestApplyRoutePreviewToScheduleRejectsNegativeValues(t *testing.T) {
	schedule := &dbmodel.Schedule{}
	distance := -1
	_, err := applyRoutePreviewToSchedule(schedule, &integrationScheduleRoutePreviewRequest{
		DistanceMeters: &distance,
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
