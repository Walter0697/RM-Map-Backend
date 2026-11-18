package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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

const (
	googlePlacesOutcomeQuota     = "quota_or_rate_limit"
	googlePlacesOutcomeBilling   = "billing_or_api_key"
	googlePlacesOutcomeHTTP      = "http_error"
	googlePlacesOutcomeTransport = "transport_error"
	googlePlacesOutcomeDecode    = "decode_error"
	googlePlacesOutcomeConfig    = "configuration_error"
)

type googlePlacesError struct {
	outcome    string
	apiStatus  string
	httpStatus int
	err        error
}

func (err *googlePlacesError) Error() string {
	if err.err != nil {
		return err.err.Error()
	}
	if err.httpStatus > 0 {
		return fmt.Sprintf("Google Places returned HTTP %d", err.httpStatus)
	}
	return "Google Places request failed"
}

func (err *googlePlacesError) Unwrap() error { return err.err }

func googlePlacesOutcome(err error) string {
	if err == nil {
		return ""
	}
	var providerErr *googlePlacesError
	if errors.As(err, &providerErr) && providerErr.outcome != "" {
		return providerErr.outcome
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "429") || strings.Contains(message, "resource_exhausted") || strings.Contains(message, "rate_limit") || strings.Contains(message, "dailylimitexceeded") {
		return googlePlacesOutcomeQuota
	}
	if strings.Contains(message, "permission_denied") || strings.Contains(message, "request_denied") || strings.Contains(message, "api key") || strings.Contains(message, "billing") {
		return googlePlacesOutcomeBilling
	}
	return googlePlacesOutcomeHTTP
}

func googlePlacesFailureAttributes(err error) (string, string, string) {
	var providerErr *googlePlacesError
	if errors.As(err, &providerErr) {
		status := ""
		if providerErr.httpStatus > 0 {
			status = strconv.Itoa(providerErr.httpStatus)
		}
		return providerErr.outcome, status, providerErr.apiStatus
	}
	return googlePlacesOutcome(err), "", ""
}

func recordGooglePlacesFailure(span SpanHandle, err error, message string) {
	outcome, httpStatus, apiStatus := googlePlacesFailureAttributes(err)
	if outcome != "" {
		span.SetAttribute("google.places.outcome", outcome)
	}
	if httpStatus != "" {
		span.SetAttribute("google.places.http_status", httpStatus)
	}
	if apiStatus != "" {
		span.SetAttribute("google.places.api_status", apiStatus)
	}
	span.RecordError(err)
	span.SetStatus(false, message)
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
	tracer := socialAnalysisTracerFn()
	queryDigest := sha256.Sum256([]byte(query))
	traceCtx, span := tracer.Start(ctx, "google_places.search", map[string]string{
		"provider.name":         "google_places",
		"provider.operation":    "search",
		"provider.query_sha256": fmt.Sprintf("%x", queryDigest[:]),
	})
	defer span.End()
	if query == "" {
		err := &googlePlacesError{outcome: googlePlacesOutcomeConfig, err: fmt.Errorf("Google Places query is required")}
		recordGooglePlacesFailure(span, err, "Google Places query is required")
		return nil, err
	}
	if provider.baseURL == "" {
		err := &googlePlacesError{outcome: googlePlacesOutcomeConfig, err: fmt.Errorf("Google Places configuration is incomplete")}
		recordGooglePlacesFailure(span, err, "Google Places configuration is incomplete")
		return nil, err
	}
	if provider.secrets == nil {
		err := &googlePlacesError{outcome: googlePlacesOutcomeConfig, err: fmt.Errorf("Google Places secret resolver is required")}
		recordGooglePlacesFailure(span, err, "Google Places secret resolver is required")
		return nil, err
	}
	apiKey, err := provider.secrets.Resolve(ctx, "GOOGLE_MAPS_API_KEY")
	if err != nil {
		providerErr := &googlePlacesError{outcome: googlePlacesOutcomeBilling, err: fmt.Errorf("Google Maps API key unavailable: %w", err)}
		recordGooglePlacesFailure(span, providerErr, "Google Maps API key unavailable")
		return nil, providerErr
	}
	if strings.TrimSpace(apiKey) == "" {
		err := fmt.Errorf("Google Maps API key is not configured")
		recordGooglePlacesFailure(span, &googlePlacesError{outcome: googlePlacesOutcomeBilling, err: err}, "Google Maps API key is not configured")
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
		providerErr := &googlePlacesError{outcome: googlePlacesOutcomeTransport, err: fmt.Errorf("Google Places request failed: %w", err)}
		recordGooglePlacesFailure(span, providerErr, "Google Places request failed")
		return nil, providerErr
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, googlePlacesMaxBytes+1))
		apiStatus := googlePlacesAPIStatus(payload)
		outcome := classifyGooglePlacesHTTPError(response.StatusCode, apiStatus)
		providerErr := &googlePlacesError{
			outcome:    outcome,
			apiStatus:  apiStatus,
			httpStatus: response.StatusCode,
			err:        fmt.Errorf("Google Places returned HTTP %d", response.StatusCode),
		}
		if readErr != nil {
			providerErr.err = fmt.Errorf("Google Places returned HTTP %d; read error response: %w", response.StatusCode, readErr)
		}
		recordGooglePlacesFailure(span, providerErr, "Google Places returned an error")
		return nil, providerErr
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, googlePlacesMaxBytes+1))
	if err != nil {
		providerErr := &googlePlacesError{outcome: googlePlacesOutcomeTransport, err: fmt.Errorf("read Google Places response: %w", err)}
		recordGooglePlacesFailure(span, providerErr, "Google Places response read failed")
		return nil, providerErr
	}
	if len(payload) > googlePlacesMaxBytes {
		err := fmt.Errorf("Google Places response exceeds size limit")
		recordGooglePlacesFailure(span, &googlePlacesError{outcome: googlePlacesOutcomeDecode, err: err}, "Google Places response exceeded size limit")
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
		recordGooglePlacesFailure(span, &googlePlacesError{outcome: googlePlacesOutcomeDecode, err: err}, "Google Places response decode failed")
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
	span.SetAttribute("google.places.outcome", "success")
	span.SetStatus(true, "Google Places search completed")
	return places, nil
}

func googlePlacesAPIStatus(payload []byte) string {
	var decoded struct {
		Error struct {
			Status string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(payload, &decoded) != nil {
		return ""
	}
	return strings.TrimSpace(decoded.Error.Status)
}

func classifyGooglePlacesHTTPError(httpStatus int, apiStatus string) string {
	status := strings.ToLower(strings.TrimSpace(apiStatus))
	if httpStatus == http.StatusTooManyRequests || status == "resource_exhausted" || strings.Contains(status, "rate_limit") || strings.Contains(status, "dailylimitexceeded") || strings.Contains(status, "quota") {
		return googlePlacesOutcomeQuota
	}
	if httpStatus == http.StatusForbidden || status == "permission_denied" || status == "request_denied" || strings.Contains(status, "api_key") || strings.Contains(status, "billing") {
		return googlePlacesOutcomeBilling
	}
	return googlePlacesOutcomeHTTP
}
