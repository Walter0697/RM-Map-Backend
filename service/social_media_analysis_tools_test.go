package service

import "testing"

func TestResolveLocationCandidateUsesGeocoderAndPreservesConfidence(t *testing.T) {
	original := socialMediaGeocodeAddressFn
	t.Cleanup(func() { socialMediaGeocodeAddressFn = original })
	socialMediaGeocodeAddressFn = func(streetNumber, streetName, country string) (float64, float64, error) {
		if streetNumber != "123" || streetName != "Main Street" || country != "Canada" {
			t.Fatalf("unexpected geocode input: %q %q %q", streetNumber, streetName, country)
		}
		return 43.6532, -79.3832, nil
	}

	resolved, err := ResolveLocationCandidate(LocationCandidate{
		Address:    "123 Main Street, Canada",
		Confidence: 0.82,
	})
	if err != nil {
		t.Fatalf("resolve candidate: %v", err)
	}
	if resolved.Latitude != 43.6532 || resolved.Longitude != -79.3832 {
		t.Fatalf("unexpected coordinates: %#v", resolved)
	}
	if resolved.Confidence != 0.82 {
		t.Fatalf("expected confidence to be preserved, got %v", resolved.Confidence)
	}
}

func TestResolveLocationCandidateRejectsMissingAddress(t *testing.T) {
	if _, err := ResolveLocationCandidate(LocationCandidate{}); err == nil {
		t.Fatal("expected missing-address error")
	}
}
