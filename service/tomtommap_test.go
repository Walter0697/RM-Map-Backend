package service

import "testing"

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
