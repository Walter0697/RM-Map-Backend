package service

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

var (
	socialMediaAnalysisReviewCurrentUserFn     = currentUserFromRequest
	socialMediaAnalysisReviewCurrentRelationFn = GetCurrentRelation
	listPendingSocialMediaAnalysisFn           = ListPendingSocialMediaAnalysis
	approvePendingSocialMediaAnalysisFn        = ApprovePendingSocialMediaAnalysis
	rejectPendingSocialMediaAnalysisFn         = RejectPendingSocialMediaAnalysis
)

func ListPendingSocialMediaAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	user := socialMediaAnalysisReviewCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	relation, err := socialMediaAnalysisReviewCurrentRelationFn(*user)
	if err != nil {
		http.Error(w, "failed to resolve current relation", http.StatusInternalServerError)
		return
	}
	if relation == nil || relation.ID == 0 {
		respondJSON(w, http.StatusOK, PendingSocialMediaAnalysisListResponse{Items: []PendingSocialMediaAnalysisItem{}})
		return
	}
	items, err := listPendingSocialMediaAnalysisFn(r.Context(), relation.ID)
	if err != nil {
		http.Error(w, "failed to load pending analyses", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, PendingSocialMediaAnalysisListResponse{Count: len(items), Items: items})
}

func resolveSocialMediaAnalysisReviewRelation(w http.ResponseWriter, r *http.Request) (uint, bool) {
	user := socialMediaAnalysisReviewCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return 0, false
	}
	relation, err := socialMediaAnalysisReviewCurrentRelationFn(*user)
	if err != nil {
		http.Error(w, "failed to resolve current relation", http.StatusInternalServerError)
		return 0, false
	}
	if relation == nil || relation.ID == 0 {
		http.Error(w, "a current relation is required", http.StatusBadRequest)
		return 0, false
	}
	return relation.ID, true
}

func ApprovePendingSocialMediaAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	relationID, ok := resolveSocialMediaAnalysisReviewRelation(w, r)
	if !ok {
		return
	}
	job, err := approvePendingSocialMediaAnalysisFn(r.Context(), chi.URLParam(r, "id"), relationID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	respondJSON(w, http.StatusOK, socialMediaAnalysisResponse(job))
}

func RejectPendingSocialMediaAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	relationID, ok := resolveSocialMediaAnalysisReviewRelation(w, r)
	if !ok {
		return
	}
	job, err := rejectPendingSocialMediaAnalysisFn(r.Context(), chi.URLParam(r, "id"), relationID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	respondJSON(w, http.StatusOK, socialMediaAnalysisResponse(job))
}
