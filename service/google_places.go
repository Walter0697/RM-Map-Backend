package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mapmarker/backend/config"
)

const (
	googlePlacesFieldMask = "places.id,places.displayName,places.formattedAddress,places.location,places.types,places.googleMapsUri"
	googlePlacesMaxBytes  = 2 * 1024 * 1024
)

type GooglePlacesProvider interface {
	Search(context.Context, string) ([]GooglePlaceCandidate, error)
}

type GooglePlaceCandidate struct {
	PlaceID       string
	Name          string
	Address       string
	Latitude      float64
	Longitude     float64
	Types         []string
	GoogleMapsURI string
}

type googlePlacesProvider struct {
	baseURL    string
	timeout    time.Duration
	maxResults int
	secrets    SecretResolver
	http       *http.Client
}

func NewGooglePlacesProvider(setting config.GoogleMapsSetting, secrets SecretResolver, client *http.Client) GooglePlacesProvider {
	if client == nil {
		client = &http.Client{}
	}
	timeout := time.Duration(setting.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	maxResults := setting.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	return googlePlacesProvider{
		baseURL:    strings.TrimRight(strings.TrimSpace(setting.BaseURL), "/"),
		timeout:    timeout,
		maxResults: maxResults,
		secrets:    secrets,
		http:       client,
	}
}

func (provider googlePlacesProvider) Search(ctx context.Context, query string) ([]GooglePlaceCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("Google Places query is required")
	}
	if provider.baseURL == "" {
		return nil, fmt.Errorf("Google Places configuration is incomplete")
	}
	if provider.secrets == nil {
		return nil, fmt.Errorf("Google Places secret resolver is required")
	}
	tracer := socialAnalysisTracerFn()
	queryDigest := sha256.Sum256([]byte(query))
	traceCtx, span := tracer.Start(ctx, "google_places.search", map[string]string{
		"provider.name":         "google_places",
		"provider.operation":    "search",
		"provider.query_sha256": fmt.Sprintf("%x", queryDigest[:]),
	})
	defer span.End()
	apiKey, err := provider.secrets.Resolve(ctx, "GOOGLE_MAPS_API_KEY")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(false, "Google Maps API key unavailable")
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		err := fmt.Errorf("Google Maps API key is not configured")
		span.RecordError(err)
		span.SetStatus(false, "Google Maps API key is not configured")
		return nil, err
	}

	requestBody, err := json.Marshal(map[string]any{
		"textQuery": query,
		"pageSize":  provider.maxResults,
	})
	if err != nil {
		return nil, fmt.Errorf("encode Google Places request: %w", err)
	}
	requestContext, cancel := context.WithTimeout(traceCtx, provider.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, provider.baseURL+"/v1/places:searchText", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create Google Places request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Goog-Api-Key", apiKey)
	request.Header.Set("X-Goog-FieldMask", googlePlacesFieldMask)

	response, err := provider.http.Do(request)
	if err != nil {
		err = fmt.Errorf("Google Places request failed: %w", err)
		span.RecordError(err)
		span.SetStatus(false, "Google Places request failed")
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("Google Places returned HTTP %d", response.StatusCode)
		span.RecordError(err)
		span.SetStatus(false, "Google Places returned an error")
		return nil, err
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, googlePlacesMaxBytes+1))
	if err != nil {
		err = fmt.Errorf("read Google Places response: %w", err)
		span.RecordError(err)
		span.SetStatus(false, "Google Places response read failed")
		return nil, err
	}
	if len(payload) > googlePlacesMaxBytes {
		err := fmt.Errorf("Google Places response exceeds size limit")
		span.RecordError(err)
		span.SetStatus(false, "Google Places response exceeded size limit")
		return nil, err
	}
	var decoded struct {
		Places []struct {
			ID          string `json:"id"`
			DisplayName struct {
				Text string `json:"text"`
			} `json:"displayName"`
			FormattedAddress string `json:"formattedAddress"`
			Location         struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"location"`
			Types         []string `json:"types"`
			GoogleMapsURI string   `json:"googleMapsUri"`
		} `json:"places"`
	}
	if len(payload) == 0 || json.Unmarshal(payload, &decoded) != nil {
		err := fmt.Errorf("decode Google Places response")
		span.RecordError(err)
		span.SetStatus(false, "Google Places response decode failed")
		return nil, err
	}
	places := make([]GooglePlaceCandidate, 0, len(decoded.Places))
	for _, place := range decoded.Places {
		if strings.TrimSpace(place.ID) == "" || strings.TrimSpace(place.DisplayName.Text) == "" {
			continue
		}
		places = append(places, GooglePlaceCandidate{
			PlaceID:       strings.TrimSpace(place.ID),
			Name:          strings.TrimSpace(place.DisplayName.Text),
			Address:       strings.TrimSpace(place.FormattedAddress),
			Latitude:      place.Location.Latitude,
			Longitude:     place.Location.Longitude,
			Types:         place.Types,
			GoogleMapsURI: strings.TrimSpace(place.GoogleMapsURI),
		})
	}
	span.SetAttribute("result.candidate_count", strconv.Itoa(len(places)))
	span.SetStatus(true, "Google Places search completed")
	return places, nil
}
