package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mapmarker/backend/database/dbmodel"
)

func resetRoutePreviewHTTPHooks() {
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	integrationCreateAuditLogFn = CreateAPIKeyAuditLog
	integrationPlanRouteFn = PlanTomTomRouteWithETA
	integrationGenerateRouteImageFn = GenerateTomTomRouteStaticImage
}

func TestIntegrationPlanRouteHandlerMapsValidationError(t *testing.T) {
	resetRoutePreviewHTTPHooks()
	defer resetRoutePreviewHTTPHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 3}}, Name: "route-key"}, true
	}
	integrationPlanRouteFn = func(ctx context.Context, origin RouteCoordinate, destination RouteCoordinate) (*RoutePlanResult, error) {
		return nil, &RouteServiceError{
			Code:       RouteErrorInvalidInput,
			Message:    "latitude must be between -90 and 90",
			HTTPStatus: http.StatusBadRequest,
		}
	}
	integrationCreateAuditLogFn = func(apiKeyID *uint, apiKeyName string, event APIKeyAuditEvent) error { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/routes/plan", bytes.NewBufferString(`{"origin":{"lat":120,"lon":114.1},"destination":{"lat":22.3,"lon":114.2}}`))
	request.Header.Set("X-Request-ID", "req-route-1")
	IntegrationPlanRouteHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	response := integrationErrorResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected json error response: %v", err)
	}
	if response.Code != RouteErrorInvalidInput {
		t.Fatalf("expected %s, got %s", RouteErrorInvalidInput, response.Code)
	}
}

func TestIntegrationPlanRouteHandlerSuccess(t *testing.T) {
	resetRoutePreviewHTTPHooks()
	defer resetRoutePreviewHTTPHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 4}}, Name: "route-key"}, true
	}
	integrationPlanRouteFn = func(ctx context.Context, origin RouteCoordinate, destination RouteCoordinate) (*RoutePlanResult, error) {
		walkSeconds := 900
		return &RoutePlanResult{
			Origin:         origin,
			Destination:    destination,
			DistanceMeters: 3500,
			Geometry:       "22.3,114.1;22.3,114.2",
			ETA: map[string]RouteModeETA{
				RouteModeWalking: {
					Available: true,
					Seconds:   &walkSeconds,
				},
			},
		}, nil
	}
	integrationCreateAuditLogFn = func(apiKeyID *uint, apiKeyName string, event APIKeyAuditEvent) error { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/routes/plan", bytes.NewBufferString(`{"origin":{"lat":22.3,"lon":114.1},"destination":{"lat":22.4,"lon":114.2}}`))
	request.Header.Set("X-Request-ID", "req-route-2")
	IntegrationPlanRouteHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	response := integrationRoutePlanResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if response.RequestID != "req-route-2" {
		t.Fatalf("expected request_id req-route-2, got %s", response.RequestID)
	}
	if response.Route == nil || response.Route.DistanceMeters != 3500 {
		t.Fatalf("unexpected route payload: %+v", response.Route)
	}
}

func TestIntegrationGenerateRouteStaticImageHandlerSuccess(t *testing.T) {
	resetRoutePreviewHTTPHooks()
	defer resetRoutePreviewHTTPHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 5}}, Name: "route-image-key"}, true
	}
	integrationGenerateRouteImageFn = func(ctx context.Context, input RoutePreviewImageInput) (*RoutePreviewImageResult, error) {
		return &RoutePreviewImageResult{
			Image:    []byte("png-route-image"),
			MimeType: "image/png",
			Format:   "png",
			Width:    900,
			Height:   540,
		}, nil
	}
	integrationCreateAuditLogFn = func(apiKeyID *uint, apiKeyName string, event APIKeyAuditEvent) error { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/routes/static-image", bytes.NewBufferString(`{"request_id":"route-image-1","route":{"origin":{"lat":22.3,"lon":114.1},"destination":{"lat":22.4,"lon":114.2},"geometry":"22.3,114.1;22.4,114.2"}}`))
	IntegrationGenerateRouteStaticImageHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	response := integrationRouteStaticImageResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json: %v", err)
	}
	if response.RequestID != "route-image-1" {
		t.Fatalf("expected request_id route-image-1, got %s", response.RequestID)
	}
	if response.ImageBase64 == "" {
		t.Fatalf("expected image_base64 to be populated")
	}
}
