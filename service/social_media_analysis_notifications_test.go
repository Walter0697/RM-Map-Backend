package service

import (
	"strings"
	"testing"

	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
)

func TestBuildSocialMediaAnalysisConflictNotificationIncludesReviewLink(t *testing.T) {
	original := config.Data.TelegramNotification.FrontendURL
	config.Data.TelegramNotification.FrontendURL = "https://home-affine.waltercheng.com/"
	t.Cleanup(func() { config.Data.TelegramNotification.FrontendURL = original })

	message, ok := buildSocialMediaAnalysisNotification(&dbmodel.SocialMediaAnalysisJob{JobID: "analysis_123", SourceURL: "https://example.com/post"}, socialMediaAnalysisNotificationConflict)
	if !ok {
		t.Fatal("expected conflict notification")
	}
	if !strings.Contains(message, "https://home-affine.waltercheng.com/pending-social-analysis?job_id=analysis_123") {
		t.Fatalf("message did not include review link: %s", message)
	}
}

func TestBuildSocialMediaAnalysisFailedNotificationIncludesSourceAndCode(t *testing.T) {
	message, ok := buildSocialMediaAnalysisNotification(&dbmodel.SocialMediaAnalysisJob{SourceURL: "https://example.com/post", ErrorCode: "source_fetch_failed"}, socialMediaAnalysisNotificationFailed)
	if !ok {
		t.Fatal("expected failed notification")
	}
	if !strings.Contains(message, "source_fetch_failed") || !strings.Contains(message, "https://example.com/post") {
		t.Fatalf("message missing failure details: %s", message)
	}
}
