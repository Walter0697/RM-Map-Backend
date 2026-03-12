package service

import (
	"context"
	"testing"
)

func TestPlanTomTomRouteWithETARejectsInvalidCoordinates(t *testing.T) {
	_, err := PlanTomTomRouteWithETA(context.Background(), RouteCoordinate{Lat: 120, Lon: 10}, RouteCoordinate{Lat: 10, Lon: 20})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	routeErr, ok := err.(*RouteServiceError)
	if !ok {
		t.Fatalf("expected RouteServiceError, got %T", err)
	}
	if routeErr.Code != RouteErrorInvalidInput {
		t.Fatalf("expected code %s, got %s", RouteErrorInvalidInput, routeErr.Code)
	}
}

func TestPlanTomTomRouteWithETAMarksUnavailableModes(t *testing.T) {
	originalFetchFn := fetchTomTomRouteFn
	defer func() { fetchTomTomRouteFn = originalFetchFn }()

	fetchTomTomRouteFn = func(ctx context.Context, operation string, travelMode string, origin RouteCoordinate, destination RouteCoordinate) (*tomTomRouteCallResult, error) {
		switch operation {
		case "route_plan_base":
			return &tomTomRouteCallResult{
				DistanceMeters: 3200,
				TravelSeconds:  600,
				Geometry:       "22.30,114.17;22.31,114.18",
			}, nil
		case "route_plan_walking":
			return &tomTomRouteCallResult{DistanceMeters: 3200, TravelSeconds: 900}, nil
		default:
			return nil, &RouteServiceError{
				Code:       RouteErrorUpstreamFail,
				Message:    "route mode is unavailable",
				HTTPStatus: 200,
			}
		}
	}

	result, err := PlanTomTomRouteWithETA(context.Background(), RouteCoordinate{Lat: 22.3, Lon: 114.17}, RouteCoordinate{Lat: 22.31, Lon: 114.18})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.DistanceMeters != 3200 {
		t.Fatalf("expected distance 3200, got %d", result.DistanceMeters)
	}
	if result.ETA[RouteModeWalking].Available != true {
		t.Fatalf("expected walking mode to be available")
	}
	if result.ETA[RouteModeBus].Available {
		t.Fatalf("expected bus mode unavailable")
	}
	if result.ETA[RouteModePublicTransit].Available {
		t.Fatalf("expected public transit mode unavailable")
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(result.Warnings))
	}
}

func TestPlanTomTomRouteWithETARetriesTransientBaseFailure(t *testing.T) {
	originalFetchFn := fetchTomTomRouteFn
	defer func() { fetchTomTomRouteFn = originalFetchFn }()

	attempts := 0
	fetchTomTomRouteFn = func(ctx context.Context, operation string, travelMode string, origin RouteCoordinate, destination RouteCoordinate) (*tomTomRouteCallResult, error) {
		if operation == "route_plan_base" {
			attempts++
			if attempts == 1 {
				return nil, &RouteServiceError{
					Code:       RouteErrorUpstreamRetry,
					Message:    "temporary failure",
					HTTPStatus: 502,
					Retryable:  true,
				}
			}
		}
		return &tomTomRouteCallResult{
			DistanceMeters: 800,
			TravelSeconds:  120,
			Geometry:       "22.30,114.17;22.30,114.18",
		}, nil
	}

	result, err := PlanTomTomRouteWithETA(context.Background(), RouteCoordinate{Lat: 22.3, Lon: 114.17}, RouteCoordinate{Lat: 22.30, Lon: 114.18})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.DistanceMeters != 800 {
		t.Fatalf("expected distance 800, got %d", result.DistanceMeters)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
