package service

import (
	"encoding/json"
	"fmt"
	"log"
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

var getTomTomMapRequestFn = GetRequest

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
	queryCandidates := buildGeocodeQueryCandidates(streetNumber, streetName, country)
	if len(queryCandidates) == 0 {
		return 0, 0, fmt.Errorf("street_name and country are required")
	}
	countrySet := inferCountrySet(country)

	for idx, query := range queryCandidates {
		log.Printf("integration geocode attempt=%d/%d query=%q country_set=%q", idx+1, len(queryCandidates), query, countrySet)
		requestURL := buildGeocodeRequestURL(query, countrySet)
		body, err := getTomTomMapRequestFn(requestURL)
		if err != nil {
			return 0, 0, fmt.Errorf(
				"tomtom geocode request failed (attempt %d/%d, query=%q, country_set=%q): %w",
				idx+1,
				len(queryCandidates),
				query,
				countrySet,
				err,
			)
		}

		response := TomTomGeocodeResponse{}
		if err := json.Unmarshal(body, &response); err != nil {
			return 0, 0, fmt.Errorf(
				"tomtom geocode response decode failed (attempt %d/%d, query=%q, country_set=%q): %w",
				idx+1,
				len(queryCandidates),
				query,
				countrySet,
				err,
			)
		}
		if len(response.Results) == 0 {
			log.Printf("integration geocode no_results attempt=%d/%d query=%q country_set=%q", idx+1, len(queryCandidates), query, countrySet)
			continue
		}

		log.Printf(
			"integration geocode success attempt=%d/%d query=%q country_set=%q lat=%f lon=%f",
			idx+1,
			len(queryCandidates),
			query,
			countrySet,
			response.Results[0].Position.Lat,
			response.Results[0].Position.Lon,
		)
		return response.Results[0].Position.Lat, response.Results[0].Position.Lon, nil
	}

	return 0, 0, fmt.Errorf(
		"no geocode results after %d attempt(s); country_set=%q; queries=%q",
		len(queryCandidates),
		countrySet,
		strings.Join(queryCandidates, " || "),
	)
}

func buildGeocodeRequestURL(query string, countrySet string) string {
	encodedQuery := url.PathEscape(strings.TrimSpace(query))
	requestURL := constant.TomtomMapAPI + GeocodeURL + "/" + encodedQuery + ".json?key=" + config.Data.APIKEY.TomTomMap + "&limit=1"
	countrySet = strings.TrimSpace(countrySet)
	if countrySet != "" {
		requestURL += "&countrySet=" + url.QueryEscape(countrySet)
	}
	return requestURL
}

func buildGeocodeQueryCandidates(streetNumber string, streetName string, country string) []string {
	streetNumber = strings.TrimSpace(streetNumber)
	streetName = strings.TrimSpace(streetName)
	country = strings.TrimSpace(country)

	base := make([]string, 0, 4)
	if streetNumber != "" && streetName != "" && country != "" {
		base = append(base, strings.Join([]string{streetNumber, streetName, country}, ", "))
	}
	if streetName != "" && country != "" {
		base = append(base, strings.Join([]string{streetName, country}, ", "))
	}
	if streetNumber != "" && streetName != "" {
		base = append(base, strings.TrimSpace(streetNumber+" "+streetName))
	}
	countryTail := countryLastPart(country)
	if streetName != "" && countryTail != "" && countryTail != country {
		base = append(base, strings.Join([]string{streetName, countryTail}, ", "))
	}

	candidates := make([]string, 0, len(base)*2)
	seen := make(map[string]struct{}, len(base)*2)
	for _, item := range base {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			candidates = append(candidates, trimmed)
		}

		normalized := normalizeGeocodeQuery(trimmed)
		if normalized == "" || normalized == trimmed {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		candidates = append(candidates, normalized)
	}

	return candidates
}

func normalizeGeocodeQuery(value string) string {
	replacer := strings.NewReplacer(
		"ā", "a",
		"ē", "e",
		"ī", "i",
		"ō", "o",
		"ū", "u",
		"Ā", "A",
		"Ē", "E",
		"Ī", "I",
		"Ō", "O",
		"Ū", "U",
		"’", "'",
		"‐", "-",
		"‑", "-",
		"–", "-",
		"—", "-",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func countryLastPart(country string) string {
	parts := strings.Split(country, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func inferCountrySet(country string) string {
	normalized := strings.ToLower(strings.TrimSpace(country))
	if normalized == "" {
		return ""
	}

	switch {
	case strings.Contains(normalized, "japan"), strings.Contains(country, "日本"):
		return "JP"
	case strings.Contains(normalized, "hong kong"), strings.Contains(country, "香港"):
		return "HK"
	case strings.Contains(normalized, "canada"):
		return "CA"
	case strings.Contains(normalized, "united states"), strings.Contains(normalized, "usa"):
		return "US"
	}

	return ""
}
