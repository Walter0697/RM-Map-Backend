package service

import (
	"mapmarker/backend/database/dbmodel"
	"testing"
)

func TestApplyAgentAnalysisResultMapsRecommendationToJobStatus(t *testing.T) {
	tests := []struct {
		name           string
		recommendation string
		wantStatus     string
	}{
		{name: "complete", recommendation: AgentRecommendationCompleted, wantStatus: dbmodel.SocialMediaAnalysisJobStatusCompleted},
		{name: "needs input", recommendation: AgentRecommendationNeedsInput, wantStatus: dbmodel.SocialMediaAnalysisJobStatusNeedsInput},
		{name: "confirmation", recommendation: AgentRecommendationAwaitingConfirmation, wantStatus: dbmodel.SocialMediaAnalysisJobStatusAwaitingConfirmation},
		{name: "create requires confirmation", recommendation: AgentRecommendationCreateMarker, wantStatus: dbmodel.SocialMediaAnalysisJobStatusAwaitingConfirmation},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			job := dbmodel.NewSocialMediaAnalysisJob("https://example.com/post")
			if err := ApplyAgentAnalysisResult(job, AgentAnalysisResult{Recommendation: test.recommendation, Confidence: 0.9}); err != nil {
				t.Fatalf("apply result: %v", err)
			}
			if job.Status != test.wantStatus {
				t.Fatalf("expected status %q, got %q", test.wantStatus, job.Status)
			}
		})
	}
}
