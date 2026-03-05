package service

import (
	"encoding/json"
	"fmt"
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"net/url"
	"strings"
)

type AddressInfo struct {
	Country                     string `json:"country"`
	CountryCode                 string `json:"countryCode"`
	LocalName                   string `json:"localName"`
	Municipality                string `json:"municipality"`
	MunicipalitySubdivision     string `json:"municipalitySubdivision"`
	CountrySubdivision          string `json:"countrySubdivision"`
	CountrySecondarySubdivision string `json:"countrySecondarySubdivision"`
	CountryTertiarySubdivision  string `json:"countryTertiarySubdivision"`
}

type AddressWrapper struct {
	Address AddressInfo `json:"address"`
}

type TomTomResponse struct {
	Addresses []AddressWrapper `json:"addresses"`
}

type TomTomGeocodePosition struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type TomTomGeocodeResult struct {
	Position TomTomGeocodePosition `json:"position"`
}

type TomTomGeocodeResponse struct {
	Results []TomTomGeocodeResult `json:"results"`
}

const (
	ReverseGeocodeURL string = "/2/reverseGeocode"
	GeocodeURL        string = "/2/geocode"
)

func reverseGeocodeRequest(lat, lon float64) string {
	latstr := fmt.Sprintf("%f", lat)
	lonstr := fmt.Sprintf("%f", lon)
	return constant.TomtomMapAPI + ReverseGeocodeURL + "/" + latstr + "," + lonstr + ".json?key=" + config.Data.APIKEY.TomTomMap
}

func GetReverseGeocode(lat, lon float64) (*TomTomResponse, error) {
	url := reverseGeocodeRequest(lat, lon)

	var tomtomResp TomTomResponse
	_, err := GetRequestWithExternalAPIAudit(ExternalAPIProviderTomTomMap, "reverse_geocode", url, func(body []byte) error {
		if unmarshalErr := json.Unmarshal(body, &tomtomResp); unmarshalErr != nil {
			return fmt.Errorf("decode response: %w", unmarshalErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &tomtomResp, nil
}

func ResolveCountryFields(resp *TomTomResponse) (string, string, string) {
	if resp == nil || len(resp.Addresses) == 0 {
		return "", "", ""
	}

	address := resp.Addresses[0].Address
	country := strings.TrimSpace(address.Country)
	countryCode := strings.TrimSpace(address.CountryCode)
	countryPart := firstNonEmptyString(
		address.LocalName,
		address.MunicipalitySubdivision,
		address.Municipality,
		address.CountrySecondarySubdivision,
		address.CountrySubdivision,
		address.CountryTertiarySubdivision,
		address.Country,
	)

	return country, countryCode, countryPart
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func GeocodeStreetAddress(streetNumber string, streetName string, country string) (float64, float64, error) {
	queryParts := make([]string, 0, 3)
	if strings.TrimSpace(streetNumber) != "" {
		queryParts = append(queryParts, strings.TrimSpace(streetNumber))
	}
	if strings.TrimSpace(streetName) != "" {
		queryParts = append(queryParts, strings.TrimSpace(streetName))
	}
	if strings.TrimSpace(country) != "" {
		queryParts = append(queryParts, strings.TrimSpace(country))
	}
	if len(queryParts) == 0 {
		return 0, 0, fmt.Errorf("street_name and country are required")
	}

	encodedQuery := url.PathEscape(strings.Join(queryParts, ", "))
	requestURL := constant.TomtomMapAPI + GeocodeURL + "/" + encodedQuery + ".json?key=" + config.Data.APIKEY.TomTomMap + "&limit=1"
	body, err := GetRequest(requestURL)
	if err != nil {
		return 0, 0, err
	}

	response := TomTomGeocodeResponse{}
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, 0, err
	}
	if len(response.Results) == 0 {
		return 0, 0, fmt.Errorf("no geocode results")
	}

	return response.Results[0].Position.Lat, response.Results[0].Position.Lon, nil
}
