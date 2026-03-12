package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mapmarker/backend/constant"
	"net/http"
	"strings"
	"time"
)

type integrationRoutePlanRequest struct {
	Origin      RouteCoordinate `json:"origin"`
	Destination RouteCoordinate `json:"destination"`
}

type integrationRoutePlanResponse struct {
	RequestID string           `json:"request_id"`
	Route     *RoutePlanResult `json:"route"`
}

type integrationRouteStaticImageRequest struct {
	RequestID string                 `json:"request_id,omitempty"`
	Route     RoutePreviewImageInput `json:"route"`
}

type integrationRouteStaticImageResponse struct {
	RequestID   string `json:"request_id"`
	ImageBase64 string `json:"image_base64"`
	MimeType    string `json:"mime_type"`
	Format      string `json:"format"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

var integrationPlanRouteFn = PlanTomTomRouteWithETA
var integrationGenerateRouteImageFn = GenerateTomTomRouteStaticImage

func IntegrationPlanRouteHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.routes.plan", constant.APIKeyScopeStaticPreview, "")
	if !ok {
		return
	}

	payload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request := integrationRoutePlanRequest{}
	if err := decodeStrictJSONPayload(payload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	requestID := integrationRequestID(r)
	result, err := integrationPlanRouteFn(r.Context(), request.Origin, request.Destination)
	if err != nil {
		writeRouteIntegrationError(w, requestID, err)
		_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
			Operation: "integration.routes.plan.result",
			SourceIP:  requestSourceIP(r),
			Success:   false,
			Reason:    err.Error(),
		})
		return
	}

	_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
		Operation: "integration.routes.plan.result",
		SourceIP:  requestSourceIP(r),
		Success:   true,
		Reason:    fmt.Sprintf("request_id=%s distance_meters=%d", requestID, result.DistanceMeters),
	})
	respondJSON(w, http.StatusOK, integrationRoutePlanResponse{
		RequestID: requestID,
		Route:     result,
	})
}

func IntegrationGenerateRouteStaticImageHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.routes.static_image", constant.APIKeyScopeStaticPreview, "")
	if !ok {
		return
	}

	payload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request := integrationRouteStaticImageRequest{}
	if err := decodeStrictJSONPayload(payload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = integrationRequestID(r)
	}

	result, err := integrationGenerateRouteImageFn(r.Context(), request.Route)
	if err != nil {
		writeRouteIntegrationError(w, requestID, err)
		_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
			Operation: "integration.routes.static_image.result",
			SourceIP:  requestSourceIP(r),
			Success:   false,
			Reason:    err.Error(),
		})
		return
	}

	_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
		Operation: "integration.routes.static_image.result",
		SourceIP:  requestSourceIP(r),
		Success:   true,
		Reason:    fmt.Sprintf("request_id=%s", requestID),
	})

	respondJSON(w, http.StatusOK, integrationRouteStaticImageResponse{
		RequestID:   requestID,
		ImageBase64: base64.StdEncoding.EncodeToString(result.Image),
		MimeType:    result.MimeType,
		Format:      result.Format,
		Width:       result.Width,
		Height:      result.Height,
	})
}

func integrationRequestID(r *http.Request) string {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID != "" {
		return requestID
	}
	return fmt.Sprintf("route-%d", time.Now().UTC().UnixNano())
}

func writeRouteIntegrationError(w http.ResponseWriter, requestID string, err error) {
	routeErr := &RouteServiceError{}
	if errors.As(err, &routeErr) {
		message := strings.TrimSpace(routeErr.Message)
		if requestID != "" {
			message = fmt.Sprintf("%s (request_id=%s)", message, requestID)
		}
		writeIntegrationError(w, routeErr.HTTPStatus, routeErr.Code, message)
		return
	}

	writeIntegrationError(w, http.StatusBadGateway, RouteErrorUpstreamFail, fmt.Sprintf("route provider failed (request_id=%s)", requestID))
}
