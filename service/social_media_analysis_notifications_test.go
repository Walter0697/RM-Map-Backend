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

func TestSocialMediaAnalysisFailureRecipientsIncludeActiveAdminsWithoutDuplicates(t *testing.T) {
	creator := dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 7}, Username: "walter", Role: "admin"}
	previous := listActiveSocialAnalysisAdminsFn
	previousCreator := loadSocialAnalysisCreatorFn
	listActiveSocialAnalysisAdminsFn = func() ([]dbmodel.User, error) {
		return []dbmodel.User{
			creator,
			{BaseModel: dbmodel.BaseModel{ID: 8}, Username: "rosary", Role: "admin"},
		}, nil
	}
	loadSocialAnalysisCreatorFn = func(uint) (dbmodel.User, error) { return creator, nil }
	t.Cleanup(func() {
		listActiveSocialAnalysisAdminsFn = previous
		loadSocialAnalysisCreatorFn = previousCreator
	})

	job := &dbmodel.SocialMediaAnalysisJob{CreatorUserID: &creator.ID}
	recipients, err := socialMediaAnalysisNotificationRecipients(job, socialMediaAnalysisNotificationFailed)
	if err != nil {
		t.Fatalf("socialMediaAnalysisNotificationRecipients returned error: %v", err)
	}
	if len(recipients) != 2 || recipients[0].Username != "walter" || recipients[1].Username != "rosary" {
		t.Fatalf("unexpected recipients: %#v", recipients)
	}
}

func TestSocialMediaAnalysisSuccessRecipientsOnlyIncludeCreator(t *testing.T) {
	creator := dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 7}, Username: "walter", Role: "user"}
	previous := listActiveSocialAnalysisAdminsFn
	previousCreator := loadSocialAnalysisCreatorFn
	listActiveSocialAnalysisAdminsFn = func() ([]dbmodel.User, error) {
		return []dbmodel.User{{BaseModel: dbmodel.BaseModel{ID: 8}, Username: "rosary", Role: "admin"}}, nil
	}
	loadSocialAnalysisCreatorFn = func(uint) (dbmodel.User, error) { return creator, nil }
	t.Cleanup(func() {
		listActiveSocialAnalysisAdminsFn = previous
		loadSocialAnalysisCreatorFn = previousCreator
	})

	job := &dbmodel.SocialMediaAnalysisJob{CreatorUserID: &creator.ID}
	recipients, err := socialMediaAnalysisNotificationRecipients(job, socialMediaAnalysisNotificationCreated)
	if err != nil {
		t.Fatalf("socialMediaAnalysisNotificationRecipients returned error: %v", err)
	}
	if len(recipients) != 1 || recipients[0].Username != "walter" {
		t.Fatalf("unexpected recipients: %#v", recipients)
	}
}
