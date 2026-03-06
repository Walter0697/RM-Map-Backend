package service

import (
	"encoding/json"
	"net/http"
)

type scheduleTravelAnalysisRequest struct {
	Schedules []ScheduleTravelPoint `json:"schedules"`
}

type scheduleTravelAnalysisResponse struct {
	TransitionAnalysis []ScheduleTravelTransitionAnalysis `json:"transition_analysis"`
}

func ScheduleTravelAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	request := scheduleTravelAnalysisRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Schedules == nil {
		request.Schedules = []ScheduleTravelPoint{}
	}

	analysis := BuildScheduleTransitionAnalysis(request.Schedules)
	respondJSON(w, http.StatusOK, scheduleTravelAnalysisResponse{
		TransitionAnalysis: analysis,
	})
}
