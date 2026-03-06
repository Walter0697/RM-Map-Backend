package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
)

func resetIntegrationMarkerOutcomeHooks() {
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	integrationCreateMarkerOutcomeLogFn = CreateMarkerCreationOutcomeLog
}

func TestIntegrationCreateMarkerOutcomeHandlerScopeDenied(t *testing.T) {
	resetIntegrationMarkerOutcomeHooks()
	defer resetIntegrationMarkerOutcomeHooks()

	createCalled := false
	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		if requiredScope != constant.APIKeyScopeMarkersWrite {
			t.Fatalf("expected scope %s, got %s", constant.APIKeyScopeMarkersWrite, requiredScope)
		}
		http.Error(w, "api key scope denied", http.StatusForbidden)
		return nil, false
	}
	integrationCreateMarkerOutcomeLogFn = func(input MarkerCreationOutcomeLogCreateInput) (*dbmodel.MarkerCreationOutcomeLog, error) {
		createCalled = true
		return nil, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/markers/outcomes", bytes.NewBufferString(`{"link":"https://x","status":"success"}`))
	IntegrationCreateMarkerOutcomeHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
	if createCalled {
		t.Fatalf("did not expect persistence when unauthorized")
	}
}

func TestIntegrationCreateMarkerOutcomeHandlerValidation(t *testing.T) {
	resetIntegrationMarkerOutcomeHooks()
	defer resetIntegrationMarkerOutcomeHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{}, true
	}

	cases := []struct {
		name     string
		payload  string
		wantCode string
	}{
		{
			name:     "missing link",
			payload:  `{"status":"success"}`,
			wantCode: "invalid_link",
		},
		{
			name:     "missing status",
			payload:  `{"link":"https://social.example/post/1"}`,
			wantCode: "invalid_status",
		},
		{
			name:     "invalid status",
			payload:  `{"link":"https://social.example/post/1","status":"processing"}`,
			wantCode: "invalid_status",
		},
		{
			name:     "success without marker reference",
			payload:  `{"link":"https://social.example/post/1","status":"success"}`,
			wantCode: "invalid_success_reference",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			createCalled := false
			integrationCreateMarkerOutcomeLogFn = func(input MarkerCreationOutcomeLogCreateInput) (*dbmodel.MarkerCreationOutcomeLog, error) {
				createCalled = true
				return nil, nil
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/integration/markers/outcomes", bytes.NewBufferString(tc.payload))
			IntegrationCreateMarkerOutcomeHandler(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", recorder.Code)
			}
			if createCalled {
				t.Fatalf("did not expect persistence for invalid payload")
			}

			response := integrationErrorResponse{}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("expected json error response: %v", err)
			}
			if response.Code != tc.wantCode {
				t.Fatalf("expected error code %s, got %s", tc.wantCode, response.Code)
			}
		})
	}
}

func TestIntegrationCreateMarkerOutcomeHandlerSuccess(t *testing.T) {
	resetIntegrationMarkerOutcomeHooks()
	defer resetIntegrationMarkerOutcomeHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{}, true
	}

	var captured MarkerCreationOutcomeLogCreateInput
	fixed := time.Date(2026, time.March, 5, 12, 34, 56, 0, time.UTC)
	integrationCreateMarkerOutcomeLogFn = func(input MarkerCreationOutcomeLogCreateInput) (*dbmodel.MarkerCreationOutcomeLog, error) {
		captured = input
		return &dbmodel.MarkerCreationOutcomeLog{
			BaseModel: dbmodel.BaseModel{
				ID:        99,
				CreatedAt: fixed,
			},
			Link:          input.Link,
			Status:        input.Status,
			MarkerID:      input.MarkerID,
			ExternalRunID: input.ExternalRunID,
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/markers/outcomes", bytes.NewBufferString(`{"link":" https://social.example/post/42 ","status":"SUCCESS","markerId":321,"externalRunId":"wf-2026-03-05-0001"}`))
	IntegrationCreateMarkerOutcomeHandler(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", recorder.Code)
	}
	if captured.Link != "https://social.example/post/42" {
		t.Fatalf("expected trimmed link, got %q", captured.Link)
	}
	if captured.Status != "success" {
		t.Fatalf("expected normalized status success, got %q", captured.Status)
	}
	if captured.MarkerID == nil || *captured.MarkerID != 321 {
		t.Fatalf("expected markerId 321, got %+v", captured.MarkerID)
	}
	if captured.ExternalRunID == nil || *captured.ExternalRunID != "wf-2026-03-05-0001" {
		t.Fatalf("expected externalRunId persisted, got %+v", captured.ExternalRunID)
	}

	response := integrationMarkerOutcomeResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected json response: %v", err)
	}
	if response.ID != 99 {
		t.Fatalf("expected id 99, got %d", response.ID)
	}
	if response.Status != "success" {
		t.Fatalf("expected success status, got %s", response.Status)
	}
	if response.MarkerID == nil || *response.MarkerID != 321 {
		t.Fatalf("expected markerId 321 in response, got %+v", response.MarkerID)
	}
	if response.ExternalRunID == nil || *response.ExternalRunID != "wf-2026-03-05-0001" {
		t.Fatalf("expected externalRunId in response, got %+v", response.ExternalRunID)
	}
}

func TestIntegrationCreateMarkerOutcomeHandlerFailedWithDetails(t *testing.T) {
	resetIntegrationMarkerOutcomeHooks()
	defer resetIntegrationMarkerOutcomeHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{}, true
	}

	var captured MarkerCreationOutcomeLogCreateInput
	integrationCreateMarkerOutcomeLogFn = func(input MarkerCreationOutcomeLogCreateInput) (*dbmodel.MarkerCreationOutcomeLog, error) {
		captured = input
		return &dbmodel.MarkerCreationOutcomeLog{
			BaseModel: dbmodel.BaseModel{
				ID:        120,
				CreatedAt: time.Now().UTC(),
			},
			Link:           input.Link,
			Status:         input.Status,
			FailureReason:  input.FailureReason,
			FailureMessage: input.FailureMessage,
		}, nil
	}

	payload := `{"link":"https://social.example/post/54","status":"failed","failureReason":"image_download_failed","failureMessage":"HTTP 403 from source media URL"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/markers/outcomes", bytes.NewBufferString(payload))
	IntegrationCreateMarkerOutcomeHandler(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", recorder.Code)
	}
	if captured.FailureReason == nil || *captured.FailureReason != "image_download_failed" {
		t.Fatalf("expected failure reason to be persisted, got %+v", captured.FailureReason)
	}
	if captured.FailureMessage == nil || *captured.FailureMessage != "HTTP 403 from source media URL" {
		t.Fatalf("expected failure message to be persisted, got %+v", captured.FailureMessage)
	}
}
