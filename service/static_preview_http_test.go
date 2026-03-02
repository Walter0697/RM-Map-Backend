package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
)

func resetStaticPreviewHooks() {
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	integrationGetUserPreviewPinFn = GetUserPreviewPinSelection
	integrationSetUserPreviewPinFn = SetUserPreviewPinSelection
	integrationGenerateStaticMapFn = GenerateStaticMapPreviewByUsername
	integrationGeocodeAddressFn = GeocodeStreetAddress
	integrationCreateAuditLogFn = CreateAPIKeyAuditLog
}

func TestIntegrationGenerateStaticMapPreviewHandlerAuthScopeDenied(t *testing.T) {
	resetStaticPreviewHooks()
	defer resetStaticPreviewHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		if requiredScope != constant.APIKeyScopeStaticPreview {
			t.Fatalf("expected required scope %s, got %s", constant.APIKeyScopeStaticPreview, requiredScope)
		}
		http.Error(w, "api key scope denied", http.StatusForbidden)
		return nil, false
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/static-map-preview", bytes.NewBufferString(`{"username":"alice","marker_type_name":"food","lat":22.3,"lon":114.2}`))
	IntegrationGenerateStaticMapPreviewHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when scope denied, got %d", recorder.Code)
	}
}

func TestIntegrationGenerateStaticMapPreviewHandlerErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		statusCode int
		errorCode  string
	}{
		{name: "invalid coordinates", serviceErr: ErrInvalidCoordinates, statusCode: http.StatusBadRequest, errorCode: "invalid_coordinates"},
		{name: "unknown username", serviceErr: ErrUnknownUsername, statusCode: http.StatusNotFound, errorCode: "unknown_username"},
		{name: "missing preview pin", serviceErr: ErrPreviewPinSelectionRequired, statusCode: http.StatusBadRequest, errorCode: "preview_pin_not_configured"},
		{name: "invalid preview pin", serviceErr: ErrPreviewPinInvalid, statusCode: http.StatusBadRequest, errorCode: "invalid_preview_pin"},
		{name: "tomtom failure", serviceErr: ErrTomTomStaticMap, statusCode: http.StatusBadGateway, errorCode: "tomtom_dependency_failure"},
		{name: "composition failure", serviceErr: ErrImageComposition, statusCode: http.StatusInternalServerError, errorCode: "image_composition_failure"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			resetStaticPreviewHooks()
			defer resetStaticPreviewHooks()

			integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
				return &dbmodel.APIKey{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1}}, Name: "key"}, true
			}
			integrationGenerateStaticMapFn = func(username string, markerTypeName string, lat float64, lon float64) (*staticPreviewResult, error) {
				return nil, testCase.serviceErr
			}
			integrationCreateAuditLogFn = func(apiKeyID *uint, apiKeyName string, event APIKeyAuditEvent) error { return nil }

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/integration/static-map-preview", bytes.NewBufferString(`{"username":"alice","marker_type_name":"food","lat":22.3,"lon":114.2}`))
			IntegrationGenerateStaticMapPreviewHandler(recorder, request)

			if recorder.Code != testCase.statusCode {
				t.Fatalf("expected status %d, got %d", testCase.statusCode, recorder.Code)
			}

			response := integrationErrorResponse{}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("expected json error response: %v", err)
			}
			if response.Code != testCase.errorCode {
				t.Fatalf("expected error code %s, got %s", testCase.errorCode, response.Code)
			}
		})
	}
}

func TestIntegrationGenerateStaticMapPreviewHandlerSuccess(t *testing.T) {
	resetStaticPreviewHooks()
	defer resetStaticPreviewHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1}}, Name: "key"}, true
	}
	integrationGenerateStaticMapFn = func(username string, markerTypeName string, lat float64, lon float64) (*staticPreviewResult, error) {
		return &staticPreviewResult{
			User:     &dbmodel.User{Username: username},
			Image:    []byte("png-bytes"),
			MimeType: "image/png",
			Format:   "png",
			Width:    600,
			Height:   400,
		}, nil
	}
	integrationCreateAuditLogFn = func(apiKeyID *uint, apiKeyName string, event APIKeyAuditEvent) error { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/static-map-preview", bytes.NewBufferString(`{"username":"alice","marker_type_name":"food","lat":22.3,"lon":114.2}`))
	IntegrationGenerateStaticMapPreviewHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	response := integrationStaticPreviewResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}
	if response.Username != "alice" {
		t.Fatalf("unexpected username: %s", response.Username)
	}
	if response.ImageBase64 != "cG5nLWJ5dGVz" {
		t.Fatalf("unexpected base64 payload: %s", response.ImageBase64)
	}
}

func TestIntegrationGenerateStaticMapPreviewHandlerGeocodeInput(t *testing.T) {
	resetStaticPreviewHooks()
	defer resetStaticPreviewHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1}}, Name: "key"}, true
	}
	integrationGeocodeAddressFn = func(streetNumber string, streetName string, country string) (float64, float64, error) {
		if streetName != "Nathan Road" || country != "Hong Kong" {
			t.Fatalf("unexpected geocode input: %s, %s, %s", streetNumber, streetName, country)
		}
		return 22.301, 114.172, nil
	}
	integrationGenerateStaticMapFn = func(username string, markerTypeName string, lat float64, lon float64) (*staticPreviewResult, error) {
		if lat != 22.301 || lon != 114.172 {
			t.Fatalf("expected geocoded coordinates, got %f %f", lat, lon)
		}
		return &staticPreviewResult{
			User:     &dbmodel.User{Username: username},
			Image:    []byte("png-bytes"),
			MimeType: "image/png",
			Format:   "png",
			Width:    600,
			Height:   400,
		}, nil
	}
	integrationCreateAuditLogFn = func(apiKeyID *uint, apiKeyName string, event APIKeyAuditEvent) error { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/static-map-preview", bytes.NewBufferString(`{"username":"alice","marker_type_name":"food","street_number":"100","street_name":"Nathan Road","country":"Hong Kong"}`))
	IntegrationGenerateStaticMapPreviewHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}
