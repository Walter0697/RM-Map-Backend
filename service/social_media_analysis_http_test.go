package service

import (
	"bytes"
	"context"
	"encoding/json"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

func TestCreateSocialMediaAnalysisHandlerReturnsAcceptedJob(t *testing.T) {
	originalAuth := integrationAuthenticateRequestFn
	originalCreate := createSocialMediaAnalysisFn
	t.Cleanup(func() {
		integrationAuthenticateRequestFn = originalAuth
		createSocialMediaAnalysisFn = originalCreate
	})

	integrationAuthenticateRequestFn = func(http.ResponseWriter, *http.Request, string, string, string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{RelationID: 42}, true
	}
	createSocialMediaAnalysisFn = func(context.Context, SocialMediaAnalysisRequest, SocialMediaAnalysisActor) (*dbmodel.SocialMediaAnalysisJob, error) {
		return &dbmodel.SocialMediaAnalysisJob{JobID: "analysis_123", Status: dbmodel.SocialMediaAnalysisJobStatusQueued, Stage: dbmodel.SocialMediaAnalysisJobStatusQueued}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/integration/social-media/analyses", bytes.NewBufferString(`{"url":"https://example.com/post"}`))
	recorder := httptest.NewRecorder()
	CreateSocialMediaAnalysisHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response SocialMediaAnalysisResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.JobID != "analysis_123" || response.Status != dbmodel.SocialMediaAnalysisJobStatusQueued {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestGetSocialMediaAnalysisHandlerRejectsMissingJob(t *testing.T) {
	originalAuth := integrationAuthenticateRequestFn
	originalGet := getSocialMediaAnalysisFn
	t.Cleanup(func() {
		integrationAuthenticateRequestFn = originalAuth
		getSocialMediaAnalysisFn = originalGet
	})

	integrationAuthenticateRequestFn = func(http.ResponseWriter, *http.Request, string, string, string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{RelationID: 42}, true
	}
	getSocialMediaAnalysisFn = func(context.Context, string, uint) (*dbmodel.SocialMediaAnalysisJob, error) {
		return nil, gorm.ErrRecordNotFound
	}

	request := httptest.NewRequest(http.MethodGet, "/integration/social-media/analyses/missing", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "missing")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()
	GetSocialMediaAnalysisHandler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
