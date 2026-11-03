package service

import (
	"encoding/json"
	"errors"
	"mapmarker/backend/constant"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

var createSocialMediaAnalysisFn = CreateSocialMediaAnalysis
var getSocialMediaAnalysisFn = GetSocialMediaAnalysis

func CreateSocialMediaAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.social_media_analysis.create", constant.APIKeyScopeSocialMediaAnalysis, "")
	if !ok {
		return
	}
	var request SocialMediaAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}
	request.URL = strings.TrimSpace(request.URL)
	job, err := createSocialMediaAnalysisFn(r.Context(), request, SocialMediaAnalysisActor{RelationID: apiKey.RelationID})
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_social_media_analysis_request", err.Error())
		return
	}
	respondJSON(w, http.StatusAccepted, socialMediaAnalysisResponse(job))
}

func GetSocialMediaAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.social_media_analysis.get", constant.APIKeyScopeSocialMediaAnalysis, "")
	if !ok {
		return
	}
	job, err := getSocialMediaAnalysisFn(r.Context(), chi.URLParam(r, "id"), apiKey.RelationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeIntegrationError(w, http.StatusNotFound, "analysis_not_found", "analysis job was not found")
			return
		}
		writeIntegrationError(w, http.StatusInternalServerError, "analysis_status_failed", "failed to retrieve analysis job")
		return
	}
	respondJSON(w, http.StatusOK, socialMediaAnalysisResponse(job))
}
