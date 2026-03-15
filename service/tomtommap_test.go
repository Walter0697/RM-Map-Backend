package service

import (
	"fmt"
	"strings"
	"testing"
)

func TestResolveCountryFieldsUsesLocalNameFirst(t *testing.T) {
	resp := &TomTomResponse{
		Addresses: []AddressWrapper{
			{
				Address: AddressInfo{
					Country:                 "Hong Kong",
					CountryCode:             "HK",
					LocalName:               "North Point",
					Municipality:            "Hong Kong",
					CountrySubdivision:      "Hong Kong Island",
					MunicipalitySubdivision: "Eastern District",
				},
			},
		},
	}

	country, countryCode, countryPart := ResolveCountryFields(resp)
	if country != "Hong Kong" {
		t.Fatalf("expected country=Hong Kong, got %s", country)
	}
	if countryCode != "HK" {
		t.Fatalf("expected countryCode=HK, got %s", countryCode)
	}
	if countryPart != "North Point" {
		t.Fatalf("expected countryPart=North Point, got %s", countryPart)
	}
}

func TestResolveCountryFieldsFallsBackWhenLocalNameMissing(t *testing.T) {
	resp := &TomTomResponse{
		Addresses: []AddressWrapper{
			{
				Address: AddressInfo{
					Country:                 "Canada",
					CountryCode:             "CA",
					LocalName:               "",
					MunicipalitySubdivision: "North York",
					Municipality:            "Toronto",
				},
			},
		},
	}

	_, _, countryPart := ResolveCountryFields(resp)
	if countryPart != "North York" {
		t.Fatalf("expected fallback countryPart=North York, got %s", countryPart)
	}
}

func TestResolveCountryFieldsHandlesEmptyResponse(t *testing.T) {
	country, countryCode, countryPart := ResolveCountryFields(&TomTomResponse{Addresses: []AddressWrapper{}})
	if country != "" || countryCode != "" || countryPart != "" {
		t.Fatalf("expected empty fields, got country=%s countryCode=%s countryPart=%s", country, countryCode, countryPart)
	}
}

func TestBuildGeocodeQueryCandidatesIncludesNormalizedFallback(t *testing.T) {
	queries := buildGeocodeQueryCandidates("1-chōme-3-19", "Dōjima", "Kita Ward, Osaka, Japan")
	if len(queries) == 0 {
		t.Fatalf("expected query candidates")
	}

	hasOriginal := false
	hasNormalized := false
	for _, query := range queries {
		if query == "1-chōme-3-19, Dōjima, Kita Ward, Osaka, Japan" {
			hasOriginal = true
		}
		if query == "1-chome-3-19, Dojima, Kita Ward, Osaka, Japan" {
			hasNormalized = true
		}
	}

	if !hasOriginal {
		t.Fatalf("expected original query candidate, got %v", queries)
	}
	if !hasNormalized {
		t.Fatalf("expected normalized query candidate, got %v", queries)
	}
}

func TestGeocodeStreetAddressFallsBackToNormalizedQuery(t *testing.T) {
	original := getTomTomMapRequestFn
	defer func() { getTomTomMapRequestFn = original }()

	requestedQueries := make([]string, 0, 2)
	getTomTomMapRequestFn = func(requestURL string) ([]byte, error) {
		if !strings.Contains(requestURL, "countrySet=JP") {
			return nil, fmt.Errorf("expected JP countrySet hint, got URL: %s", requestURL)
		}
		if strings.Contains(requestURL, "1-ch%C5%8Dme-3-19%2C%20D%C5%8Djima%2C%20Kita%20Ward%2C%20Osaka%2C%20Japan") {
			requestedQueries = append(requestedQueries, "original")
			return []byte(`{"results":[]}`), nil
		}
		if strings.Contains(requestURL, "1-chome-3-19%2C%20Dojima%2C%20Kita%20Ward%2C%20Osaka%2C%20Japan") {
			requestedQueries = append(requestedQueries, "normalized")
			return []byte(`{"results":[{"position":{"lat":34.69,"lon":135.50}}]}`), nil
		}
		return nil, fmt.Errorf("unexpected request URL: %s", requestURL)
	}

	lat, lon, err := GeocodeStreetAddress("1-chōme-3-19", "Dōjima", "Kita Ward, Osaka, Japan")
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if lat != 34.69 || lon != 135.50 {
		t.Fatalf("unexpected lat/lon: %f,%f", lat, lon)
	}
	if len(requestedQueries) < 2 || requestedQueries[0] != "original" || requestedQueries[1] != "normalized" {
		t.Fatalf("expected original then normalized fallback, got sequence=%v", requestedQueries)
	}
}

func TestInferCountrySet(t *testing.T) {
	if got := inferCountrySet("Kita Ward, Osaka, Japan"); got != "JP" {
		t.Fatalf("expected JP, got %q", got)
	}
	if got := inferCountrySet("香港"); got != "HK" {
		t.Fatalf("expected HK, got %q", got)
	}
	if got := inferCountrySet("Toronto, Canada"); got != "CA" {
		t.Fatalf("expected CA, got %q", got)
	}
	if got := inferCountrySet("Unknown"); got != "" {
		t.Fatalf("expected empty country set, got %q", got)
	}
}
