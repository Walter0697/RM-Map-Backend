package service

import "testing"

func TestVerifyLocationReturnsMatchForNearbySamePlace(t *testing.T) {
	result := VerifyLocation(LocationCandidate{
		Name:      "IFC Mall",
		Address:   "Central, Hong Kong",
		Latitude:  22.2854,
		Longitude: 114.1579,
	}, []GooglePlaceCandidate{{
		PlaceID:   "ifc-1",
		Name:      "IFC Mall",
		Address:   "8 Finance Street, Central, Hong Kong",
		Latitude:  22.2855,
		Longitude: 114.1579,
	}})
	if result.Status != LocationMatch || result.Candidate == nil || result.Candidate.PlaceID != "ifc-1" {
		t.Fatalf("expected strong match, got %#v", result)
	}
}

func TestVerifyLocationReturnsAmbiguousForMultiplePlausiblePlaces(t *testing.T) {
	result := VerifyLocation(LocationCandidate{Name: "Central Market", Address: "Hong Kong"}, []GooglePlaceCandidate{
		{Name: "Central Market", Address: "Central, Hong Kong", Latitude: 22.28, Longitude: 114.15},
		{Name: "Central Market", Address: "Sheung Wan, Hong Kong", Latitude: 22.28, Longitude: 114.15},
	})
	if result.Status != LocationAmbiguous {
		t.Fatalf("expected ambiguous result, got %#v", result)
	}
}

func TestVerifyLocationReturnsNoMatchForUnrelatedPlaces(t *testing.T) {
	result := VerifyLocation(LocationCandidate{Name: "IFC Mall", Address: "Hong Kong"}, []GooglePlaceCandidate{{
		Name: "Tokyo Station", Address: "Tokyo, Japan", Latitude: 35.68, Longitude: 139.76,
	}})
	if result.Status != LocationNoMatch {
		t.Fatalf("expected no match, got %#v", result)
	}
}

func TestVerifyLocationRejectsLargeCoordinateMismatch(t *testing.T) {
	result := VerifyLocation(LocationCandidate{
		Name:      "IFC Mall",
		Address:   "Central, Hong Kong",
		Latitude:  22.2854,
		Longitude: 114.1579,
	}, []GooglePlaceCandidate{{
		Name: "IFC Mall", Address: "IFC Mall, Toronto, Canada", Latitude: 43.65, Longitude: -79.38,
	}})
	if result.Status != LocationNoMatch {
		t.Fatalf("expected coordinate mismatch to be rejected, got %#v", result)
	}
}

func TestVerifyLocationHandlesEmptyResults(t *testing.T) {
	result := VerifyLocation(LocationCandidate{Name: "IFC Mall"}, nil)
	if result.Status != LocationNoMatch {
		t.Fatalf("expected empty results to be no_match, got %#v", result)
	}
}
