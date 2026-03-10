package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTomTomRouteBaseURL      = "https://api.tomtom.com/routing/1"
	defaultTomTomRouteTimeoutMS    = 3500
	defaultTomTomRouteRetryCount   = 1
	defaultTomTomStaticImageURL    = "https://api.tomtom.com/map/1/staticimage"
	defaultTomTomStaticImageWidth  = 900
	defaultTomTomStaticImageHeight = 540
	defaultTomTomStaticImageZoom   = 12
)

const (
	RouteModeWalking       = "walking"
	RouteModeBus           = "bus"
	RouteModePublicTransit = "public_transit"
)

const (
	tomTomTravelModeCar           = "car"
	tomTomTravelModePedestrian    = "pedestrian"
	tomTomTravelModeBus           = "bus"
	tomTomTravelModePublicTransit = "publicTransit"
)

const (
	RouteErrorInvalidInput  = "invalid_route_input"
	RouteErrorUpstreamRate  = "route_provider_rate_limited"
	RouteErrorUpstreamRetry = "route_provider_retriable_failure"
	RouteErrorUpstreamFail  = "route_provider_failed"
)

type RouteCoordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type RouteModeETA struct {
	Available bool   `json:"available"`
	Seconds   *int   `json:"seconds,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

type RoutePlanResult struct {
	Origin         RouteCoordinate         `json:"origin"`
	Destination    RouteCoordinate         `json:"destination"`
	DistanceMeters int                     `json:"distance_meters"`
	Geometry       string                  `json:"geometry"`
	Paths          map[string]string       `json:"paths,omitempty"`
	ETA            map[string]RouteModeETA `json:"eta"`
	Warnings       []string                `json:"warnings,omitempty"`
}

type RoutePreviewImageInput struct {
	Origin      RouteCoordinate `json:"origin"`
	Destination RouteCoordinate `json:"destination"`
	Geometry    string          `json:"geometry,omitempty"`
	DistanceM   *int            `json:"distance_meters,omitempty"`
}

type RoutePreviewImageResult struct {
	Image    []byte
	MimeType string
	Format   string
	Width    int
	Height   int
}

type RouteServiceError struct {
	Code       string
	Message    string
	HTTPStatus int
	Retryable  bool
}

func (e *RouteServiceError) Error() string {
	return fmt.Sprintf("%s: %s", strings.TrimSpace(e.Code), strings.TrimSpace(e.Message))
}

type tomTomRouteCallResult struct {
	DistanceMeters int
	TravelSeconds  int
	Geometry       string
}

type tomTomRoutePreviewResponse struct {
	Routes []struct {
		Summary struct {
			LengthInMeters      int `json:"lengthInMeters"`
			TravelTimeInSeconds int `json:"travelTimeInSeconds"`
		} `json:"summary"`
		Legs []struct {
			Points []struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"points"`
		} `json:"legs"`
	} `json:"routes"`
	ErrorText string `json:"errorText"`
}

type tomTomRouteModeConfig struct {
	PublicMode string
	TravelMode string
}

var fetchTomTomRouteFn = fetchTomTomRoute
var generateRouteStaticImageNowFn = time.Now

func PlanTomTomRouteWithETA(ctx context.Context, origin RouteCoordinate, destination RouteCoordinate) (*RoutePlanResult, error) {
	if err := validateRouteCoordinate(origin); err != nil {
		return nil, err
	}
	if err := validateRouteCoordinate(destination); err != nil {
		return nil, err
	}

	requestStart := time.Now().UTC()
	baseRoute, err := fetchTomTomRouteWithRetry(ctx, "route_plan_base", tomTomTravelModeCar, origin, destination, true)
	if err != nil {
		return nil, err
	}

	result := &RoutePlanResult{
		Origin:         origin,
		Destination:    destination,
		DistanceMeters: baseRoute.DistanceMeters,
		Geometry:       baseRoute.Geometry,
		Paths: map[string]string{
			"driving": baseRoute.Geometry,
		},
		ETA:            map[string]RouteModeETA{},
	}

	modeConfigs := []tomTomRouteModeConfig{
		{PublicMode: RouteModeWalking, TravelMode: tomTomTravelModePedestrian},
		{PublicMode: RouteModeBus, TravelMode: tomTomTravelModeBus},
		{PublicMode: RouteModePublicTransit, TravelMode: tomTomTravelModePublicTransit},
	}
	for _, modeConfig := range modeConfigs {
		modeResult, modeErr := fetchTomTomRouteWithRetry(ctx, "route_plan_"+modeConfig.PublicMode, modeConfig.TravelMode, origin, destination, false)
		if modeErr == nil {
			seconds := modeResult.TravelSeconds
			if result.Paths == nil {
				result.Paths = map[string]string{}
			}
			result.Paths[modeConfig.PublicMode] = modeResult.Geometry
			result.ETA[modeConfig.PublicMode] = RouteModeETA{
				Available: true,
				Seconds:   &seconds,
			}
			continue
		}

		routeErr, ok := modeErr.(*RouteServiceError)
		if !ok {
			result.ETA[modeConfig.PublicMode] = RouteModeETA{
				Available: false,
				Code:      RouteErrorUpstreamFail,
				Message:   "route provider failed",
			}
			result.Warnings = append(result.Warnings, modeConfig.PublicMode+" ETA is unavailable")
			continue
		}

		result.ETA[modeConfig.PublicMode] = RouteModeETA{
			Available: false,
			Code:      routeErr.Code,
			Message:   routeErr.Message,
		}
		result.Warnings = append(result.Warnings, modeConfig.PublicMode+" ETA is unavailable")
	}

	logTomTomRouteOutcome("success", requestStart, nil)
	return result, nil
}

func GenerateTomTomRouteStaticImage(ctx context.Context, input RoutePreviewImageInput) (*RoutePreviewImageResult, error) {
	if err := validateRouteCoordinate(input.Origin); err != nil {
		return nil, err
	}
	if err := validateRouteCoordinate(input.Destination); err != nil {
		return nil, err
	}

	apiKey := strings.TrimSpace(resolveTomTomStaticImageAPIKey())
	if apiKey == "" {
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "tomtom static image api key is not configured",
			HTTPStatus: http.StatusBadGateway,
		}
	}

	endpoint, err := url.Parse(resolveTomTomStaticImageBaseURL())
	if err != nil {
		return nil, err
	}

	centerLon := (input.Origin.Lon + input.Destination.Lon) / 2
	centerLat := (input.Origin.Lat + input.Destination.Lat) / 2

	query := endpoint.Query()
	query.Set("key", apiKey)
	query.Set("zoom", fmt.Sprintf("%d", defaultTomTomStaticImageZoom))
	query.Set("center", fmt.Sprintf("%.6f,%.6f", centerLon, centerLat))
	query.Set("width", fmt.Sprintf("%d", defaultTomTomStaticImageWidth))
	query.Set("height", fmt.Sprintf("%d", defaultTomTomStaticImageHeight))
	query.Set("format", "png")
	query.Set("layer", "basic")
	query.Set("style", "main")
	pathSummary := strings.TrimSpace(input.Geometry)
	if pathSummary != "" {
		query.Set("path", pathSummary)
	}
	endpoint.RawQuery = query.Encode()

	client := &http.Client{Timeout: time.Duration(resolveTomTomStaticImageTimeoutMS()) * time.Millisecond}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	requestStart := time.Now().UTC()
	response, err := client.Do(request)
	if err != nil {
		logTomTomStaticImageOutcome("error", requestStart, nil, err)
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamRetry,
			Message:    "route static image provider request failed",
			HTTPStatus: http.StatusBadGateway,
			Retryable:  true,
		}
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(response.Body)
	status := response.StatusCode
	if readErr != nil {
		logTomTomStaticImageOutcome("error", requestStart, &status, readErr)
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "failed to read route static image response",
			HTTPStatus: http.StatusBadGateway,
		}
	}

	if status == http.StatusTooManyRequests {
		logTomTomStaticImageOutcome("rate_limited", requestStart, &status, fmt.Errorf("status=%d", status))
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamRate,
			Message:    "route image provider quota exceeded",
			HTTPStatus: http.StatusTooManyRequests,
		}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices || len(body) == 0 {
		logTomTomStaticImageOutcome("error", requestStart, &status, fmt.Errorf("status=%d", status))
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "route image provider returned an invalid response",
			HTTPStatus: http.StatusBadGateway,
		}
	}

	logTomTomStaticImageOutcome("success", requestStart, &status, nil)
	return &RoutePreviewImageResult{
		Image:    body,
		MimeType: "image/png",
		Format:   "png",
		Width:    defaultTomTomStaticImageWidth,
		Height:   defaultTomTomStaticImageHeight,
	}, nil
}

func fetchTomTomRouteWithRetry(ctx context.Context, operation string, travelMode string, origin RouteCoordinate, destination RouteCoordinate, required bool) (*tomTomRouteCallResult, error) {
	retries := resolveTomTomRouteRetryCount()
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		result, err := fetchTomTomRouteFn(ctx, operation, travelMode, origin, destination)
		if err == nil {
			return result, nil
		}

		lastErr = err
		routeErr, ok := err.(*RouteServiceError)
		if !ok || !routeErr.Retryable {
			break
		}
	}
	if required {
		return nil, lastErr
	}
	return nil, lastErr
}

func fetchTomTomRoute(ctx context.Context, operation string, travelMode string, origin RouteCoordinate, destination RouteCoordinate) (*tomTomRouteCallResult, error) {
	apiKey := strings.TrimSpace(resolveTomTomRoutingAPIKey())
	if apiKey == "" {
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "tomtom route api key is not configured",
			HTTPStatus: http.StatusBadGateway,
		}
	}

	baseURL := strings.TrimSuffix(resolveTomTomRouteBaseURL(), "/")
	endpoint, err := url.Parse(fmt.Sprintf(
		"%s/calculateRoute/%.6f,%.6f:%.6f,%.6f/json",
		baseURL,
		origin.Lat,
		origin.Lon,
		destination.Lat,
		destination.Lon,
	))
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("key", apiKey)
	query.Set("traffic", "true")
	query.Set("travelMode", travelMode)
	query.Set("computeTravelTimeFor", "all")
	endpoint.RawQuery = query.Encode()

	timeout := time.Duration(resolveTomTomRouteTimeoutMS()) * time.Millisecond
	client := &http.Client{Timeout: timeout}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	requestStart := generateRouteStaticImageNowFn().UTC()
	response, err := client.Do(request)
	if err != nil {
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, nil, requestStart, err)
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamRetry,
			Message:    "route provider request failed",
			HTTPStatus: http.StatusBadGateway,
			Retryable:  true,
		}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	status := response.StatusCode
	if err != nil {
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, &status, requestStart, err)
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "failed to read route provider response",
			HTTPStatus: http.StatusBadGateway,
		}
	}

	if status == http.StatusTooManyRequests {
		log.Printf("[tomtom-route] operation=%s travel_mode=%s status=%d detail=%s", operation, travelMode, status, summarizeProviderBody(body))
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, &status, requestStart, fmt.Errorf("status=%d", status))
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamRate,
			Message:    "route provider quota exceeded",
			HTTPStatus: http.StatusTooManyRequests,
		}
	}
	if status >= http.StatusInternalServerError {
		log.Printf("[tomtom-route] operation=%s travel_mode=%s status=%d detail=%s", operation, travelMode, status, summarizeProviderBody(body))
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, &status, requestStart, fmt.Errorf("status=%d", status))
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamRetry,
			Message:    "route provider temporary failure",
			HTTPStatus: http.StatusBadGateway,
			Retryable:  true,
		}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		detail := summarizeProviderBody(body)
		log.Printf("[tomtom-route] operation=%s travel_mode=%s status=%d detail=%s", operation, travelMode, status, detail)
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, &status, requestStart, fmt.Errorf("status=%d", status))
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    fmt.Sprintf("route provider returned status %d (%s)", status, detail),
			HTTPStatus: http.StatusBadGateway,
		}
	}

	result := tomTomRoutePreviewResponse{}
	if err := json.Unmarshal(body, &result); err != nil {
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, &status, requestStart, err)
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "failed to parse route provider response",
			HTTPStatus: http.StatusBadGateway,
		}
	}
	if len(result.Routes) == 0 {
		log.Printf("[tomtom-route] operation=%s travel_mode=%s status=%d detail=empty route payload", operation, travelMode, status)
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, &status, requestStart, fmt.Errorf("empty route response"))
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "route provider returned no route",
			HTTPStatus: http.StatusBadGateway,
		}
	}

	summary := result.Routes[0].Summary
	if summary.LengthInMeters <= 0 || summary.TravelTimeInSeconds <= 0 {
		log.Printf("[tomtom-route] operation=%s travel_mode=%s status=%d detail=invalid route summary", operation, travelMode, status)
		logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusError, &status, requestStart, fmt.Errorf("missing route summary"))
		return nil, &RouteServiceError{
			Code:       RouteErrorUpstreamFail,
			Message:    "route provider summary is invalid",
			HTTPStatus: http.StatusBadGateway,
		}
	}

	logTomTomRouteAudit(operation, dbmodel.ExternalAPIAuditStatusSuccess, &status, requestStart, nil)
	return &tomTomRouteCallResult{
		DistanceMeters: summary.LengthInMeters,
		TravelSeconds:  summary.TravelTimeInSeconds,
		Geometry:       summarizeTomTomGeometry(result.Routes[0].Legs),
	}, nil
}

func BuildFallbackRoutePlan(origin RouteCoordinate, destination RouteCoordinate, reason string) *RoutePlanResult {
	distanceMeters := estimateStraightLineDistanceMeters(origin, destination)
	warning := strings.TrimSpace(reason)
	if warning == "" {
		warning = "route provider unavailable; using straight-line preview"
	}
	return &RoutePlanResult{
		Origin:         origin,
		Destination:    destination,
		DistanceMeters: distanceMeters,
		Geometry:       fmt.Sprintf("%.5f,%.5f;%.5f,%.5f", origin.Lat, origin.Lon, destination.Lat, destination.Lon),
		Paths: map[string]string{
			"driving": fmt.Sprintf("%.5f,%.5f;%.5f,%.5f", origin.Lat, origin.Lon, destination.Lat, destination.Lon),
		},
		ETA: map[string]RouteModeETA{
			RouteModeWalking: {
				Available: false,
				Code:      RouteErrorUpstreamFail,
				Message:   warning,
			},
			RouteModeBus: {
				Available: false,
				Code:      RouteErrorUpstreamFail,
				Message:   warning,
			},
			RouteModePublicTransit: {
				Available: false,
				Code:      RouteErrorUpstreamFail,
				Message:   warning,
			},
		},
		Warnings: []string{warning},
	}
}

func estimateStraightLineDistanceMeters(origin RouteCoordinate, destination RouteCoordinate) int {
	const earthRadiusMeters = 6371000.0
	toRad := func(value float64) float64 {
		return value * math.Pi / 180.0
	}
	lat1 := toRad(origin.Lat)
	lat2 := toRad(destination.Lat)
	dLat := toRad(destination.Lat - origin.Lat)
	dLon := toRad(destination.Lon - origin.Lon)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int(math.Round(earthRadiusMeters * c))
}

func summarizeProviderBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "empty response"
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err == nil {
		for _, key := range []string{"errorText", "error", "message", "detailedError"} {
			if raw, ok := parsed[key]; ok {
				text := strings.TrimSpace(fmt.Sprintf("%v", raw))
				if text != "" {
					if len(text) > 180 {
						return text[:180]
					}
					return text
				}
			}
		}
	}
	if len(trimmed) > 180 {
		return trimmed[:180]
	}
	return trimmed
}

func summarizeTomTomGeometry(legs []struct {
	Points []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"points"`
}) string {
	if len(legs) == 0 {
		return ""
	}

	result := make([]string, 0)
	for _, leg := range legs {
		for _, point := range leg.Points {
			result = append(result, fmt.Sprintf("%.5f,%.5f", point.Latitude, point.Longitude))
		}
	}
	if len(result) == 0 {
		return ""
	}
	return strings.Join(result, ";")
}

func logTomTomRouteAudit(operation string, statusClass string, httpStatus *int, requestTime time.Time, err error) {
	now := generateRouteStaticImageNowFn().UTC()
	event := ExternalAPIAuditWriteInput{
		Provider:    ExternalAPIProviderTomTomMap,
		Operation:   operation,
		StatusClass: statusClass,
		HTTPStatus:  httpStatus,
		LatencyMS:   now.Sub(requestTime).Milliseconds(),
		RequestTime: requestTime,
		CompletedAt: &now,
	}
	if err != nil {
		event.ErrorClass = classifyExternalAPIErr(err)
		event.ErrorDetail = strings.TrimSpace(err.Error())
	}
	RecordExternalAPIAuditEvent(event)
}

func logTomTomRouteOutcome(status string, requestTime time.Time, err error) {
	latency := time.Since(requestTime).Milliseconds()
	if err != nil {
		log.Printf("[tomtom-route] status=%s latency_ms=%d error=%v", strings.TrimSpace(status), latency, err)
		return
	}
	log.Printf("[tomtom-route] status=%s latency_ms=%d", strings.TrimSpace(status), latency)
}

func logTomTomStaticImageOutcome(status string, requestTime time.Time, statusCode *int, err error) {
	now := generateRouteStaticImageNowFn().UTC()
	statusClass := dbmodel.ExternalAPIAuditStatusSuccess
	if err != nil {
		statusClass = dbmodel.ExternalAPIAuditStatusError
	}

	event := ExternalAPIAuditWriteInput{
		Provider:    ExternalAPIProviderTomTomMap,
		Operation:   "route_static_image",
		StatusClass: statusClass,
		HTTPStatus:  statusCode,
		LatencyMS:   now.Sub(requestTime).Milliseconds(),
		RequestTime: requestTime,
		CompletedAt: &now,
	}
	if err != nil {
		event.ErrorClass = classifyExternalAPIErr(err)
		event.ErrorDetail = strings.TrimSpace(err.Error())
	}
	RecordExternalAPIAuditEvent(event)

	if err != nil {
		log.Printf("[tomtom-route-static-image] status=%s latency_ms=%d error=%v", status, now.Sub(requestTime).Milliseconds(), err)
		return
	}
	log.Printf("[tomtom-route-static-image] status=%s latency_ms=%d", status, now.Sub(requestTime).Milliseconds())
}

func validateRouteCoordinate(coordinate RouteCoordinate) error {
	if coordinate.Lat < -90 || coordinate.Lat > 90 {
		return &RouteServiceError{
			Code:       RouteErrorInvalidInput,
			Message:    "latitude must be between -90 and 90",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	if coordinate.Lon < -180 || coordinate.Lon > 180 {
		return &RouteServiceError{
			Code:       RouteErrorInvalidInput,
			Message:    "longitude must be between -180 and 180",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	return nil
}

func resolveTomTomRoutingAPIKey() string {
	key := strings.TrimSpace(config.Data.APIKEY.TomTomRouting)
	if key != "" {
		return key
	}
	return strings.TrimSpace(config.Data.APIKEY.TomTomMap)
}

func resolveTomTomStaticImageAPIKey() string {
	key := strings.TrimSpace(config.Data.APIKEY.TomTomStaticImage)
	if key != "" {
		return key
	}
	return strings.TrimSpace(config.Data.APIKEY.TomTomMap)
}

func resolveTomTomRouteBaseURL() string {
	baseURL := strings.TrimSpace(config.Data.TomTomRoute.BaseURL)
	if baseURL != "" {
		return baseURL
	}
	return defaultTomTomRouteBaseURL
}

func resolveTomTomStaticImageBaseURL() string {
	baseURL := strings.TrimSpace(config.Data.TomTomStaticImage.BaseURL)
	if baseURL != "" {
		return baseURL
	}
	return defaultTomTomStaticImageURL
}

func resolveTomTomRouteTimeoutMS() int {
	timeout := config.Data.TomTomRoute.TimeoutMS
	if timeout > 0 {
		return timeout
	}
	return defaultTomTomRouteTimeoutMS
}

func resolveTomTomStaticImageTimeoutMS() int {
	timeout := config.Data.TomTomStaticImage.TimeoutMS
	if timeout > 0 {
		return timeout
	}
	return int(defaultStaticPreviewTimeout.Milliseconds())
}

func resolveTomTomRouteRetryCount() int {
	retryCount := config.Data.TomTomRoute.RetryCount
	if retryCount > 0 {
		return retryCount
	}
	return defaultTomTomRouteRetryCount
}
