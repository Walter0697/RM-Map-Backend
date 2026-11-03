package dbmodel

import "testing"

func TestSocialMediaAnalysisJobDefaultsToQueued(t *testing.T) {
	job := NewSocialMediaAnalysisJob("https://example.com/post")

	if job.Status != SocialMediaAnalysisJobStatusQueued {
		t.Fatalf("expected queued status, got %q", job.Status)
	}
	if job.SourceURL != "https://example.com/post" {
		t.Fatalf("expected source URL to be retained, got %q", job.SourceURL)
	}
}

func TestSocialMediaAnalysisStatusConstantsRemainDistinct(t *testing.T) {
	statuses := []string{
		SocialMediaAnalysisJobStatusQueued,
		SocialMediaAnalysisJobStatusProcessing,
		SocialMediaAnalysisJobStatusAwaitingConfirmation,
		SocialMediaAnalysisJobStatusCompleted,
		SocialMediaAnalysisJobStatusCreated,
		SocialMediaAnalysisJobStatusNeedsInput,
		SocialMediaAnalysisJobStatusFailed,
		SocialMediaAnalysisJobStatusCancelled,
	}

	seen := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		if status == "" {
			t.Fatal("status constants must not be empty")
		}
		if _, exists := seen[status]; exists {
			t.Fatalf("duplicate status constant %q", status)
		}
		seen[status] = struct{}{}
	}
}
