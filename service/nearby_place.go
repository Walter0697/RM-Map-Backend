package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mapmarker/backend/config"
	"mapmarker/backend/constant"
)

const maxNearbyPlaceCandidates = 5
const maxNearbyPlaceResponseBytes = 256 * 1024

type nearbyPlaceCandidate struct {
	Label     string  `json:"label,omitempty"`
	Address   string  `json:"address,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

var integrationResolveNearbyPlaceFn = resolveNearbyPlace

var nearbyPlaceHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func nearbyCoordinateInRange(value, bound float64) bool {
	return !math.IsNaN(value) && value >= -bound && value <= bound
}

func resolveNearbyPlace(ctx context.Context, place, countryCode string) ([]nearbyPlaceCandidate, error) {
	key := strings.TrimSpace(config.Data.APIKEY.TomTomMap)
	if key == "" {
		return nil, fmt.Errorf("place lookup is not configured")
	}
	query := url.Values{"key": {key}, "limit": {"5"}}
	if countryCode != "" {
		query.Set("countrySet", countryCode)
	}
	endpoint := constant.TomtomMapAPI + "/2/search/" + url.PathEscape(place) + ".json?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build place lookup request")
	}
	resp, err := nearbyPlaceHTTPClient.Do(req)
	if err != nil {
		// A transport error can contain the request URL and API key.
		return nil, fmt.Errorf("place lookup request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("place lookup returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxNearbyPlaceResponseBytes+1))
	if err != nil || len(data) > maxNearbyPlaceResponseBytes {
		return nil, fmt.Errorf("could not read place lookup response")
	}
	return decodeNearbyPlaceCandidates(data)
}

func decodeNearbyPlaceCandidates(data []byte) ([]nearbyPlaceCandidate, error) {
	var response struct {
		Results *[]struct {
			Position struct {
				Lat *float64 `json:"lat"`
				Lon *float64 `json:"lon"`
			} `json:"position"`
			POI struct {
				Name string `json:"name"`
			} `json:"poi"`
			Address struct {
				Freeform string `json:"freeformAddress"`
			} `json:"address"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &response); err != nil || response.Results == nil {
		return nil, fmt.Errorf("invalid place lookup response")
	}
	places := make([]nearbyPlaceCandidate, 0, maxNearbyPlaceCandidates)
	seen := make(map[nearbyPlaceCandidate]bool, maxNearbyPlaceCandidates)
	for _, result := range *response.Results {
		lat, lon := result.Position.Lat, result.Position.Lon
		if lat == nil || lon == nil || !nearbyCoordinateInRange(*lat, 90) || !nearbyCoordinateInRange(*lon, 180) {
			continue
		}
		candidate := nearbyPlaceCandidate{
			Label:     firstNonEmptyString(result.POI.Name, result.Address.Freeform),
			Address:   strings.TrimSpace(result.Address.Freeform),
			Latitude:  *lat,
			Longitude: *lon,
		}
		if candidate.Label == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		places = append(places, candidate)
		if len(places) == maxNearbyPlaceCandidates {
			break
		}
	}
	return places, nil
}
