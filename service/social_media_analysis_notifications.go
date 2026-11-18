package service

import (
	"context"
	"fmt"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/url"
	"strings"
)

const (
	socialMediaAnalysisNotificationConflict = "conflict"
	socialMediaAnalysisNotificationCreated  = "created"
	socialMediaAnalysisNotificationFailed   = "failed"
)

var listActiveSocialAnalysisAdminsFn = listActiveSocialAnalysisAdmins
var loadSocialAnalysisCreatorFn = loadSocialAnalysisCreator

func notifySocialMediaAnalysisEvent(ctx context.Context, job *dbmodel.SocialMediaAnalysisJob, event string) {
	if job == nil || database.Connection == nil {
		return
	}
	message, ok := buildSocialMediaAnalysisNotification(job, event)
	if !ok {
		return
	}
	recipients, err := socialMediaAnalysisNotificationRecipients(job, event)
	if err != nil {
		log.Printf("social_media_analysis notification user_lookup_failed job_id=%s err=%v", job.JobID, err)
		return
	}
	for _, user := range recipients {
		if err := notifyTelegramMessageContext(ctx, user, message, event); err != nil {
			log.Printf("social_media_analysis notification failed job_id=%s event=%s actor=%s err=%v", job.JobID, event, strings.TrimSpace(user.Username), err)
		}
	}
}

func listActiveSocialAnalysisAdmins() ([]dbmodel.User, error) {
	if database.Connection == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	users := make([]dbmodel.User, 0)
	if err := database.Connection.Where("is_activated = ? AND role = ?", true, "admin").Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func socialMediaAnalysisNotificationRecipients(job *dbmodel.SocialMediaAnalysisJob, event string) ([]dbmodel.User, error) {
	if job == nil {
		return nil, nil
	}
	recipients := make([]dbmodel.User, 0, 2)
	if job.CreatorUserID != nil {
		creator, err := loadSocialAnalysisCreatorFn(*job.CreatorUserID)
		if err != nil {
			return nil, err
		}
		recipients = append(recipients, creator)
	}
	if event != socialMediaAnalysisNotificationFailed {
		return recipients, nil
	}
	admins, err := listActiveSocialAnalysisAdminsFn()
	if err != nil {
		return nil, err
	}
	for _, admin := range admins {
		duplicate := false
		for _, recipient := range recipients {
			if recipient.ID != 0 && recipient.ID == admin.ID || recipient.ID == 0 && recipient.Username == admin.Username {
				duplicate = true
				break
			}
		}
		if !duplicate {
			recipients = append(recipients, admin)
		}
	}
	return recipients, nil
}

func loadSocialAnalysisCreator(userID uint) (dbmodel.User, error) {
	var user dbmodel.User
	if err := database.Connection.First(&user, userID).Error; err != nil {
		return dbmodel.User{}, err
	}
	return user, nil
}

func buildSocialMediaAnalysisNotification(job *dbmodel.SocialMediaAnalysisJob, event string) (string, bool) {
	if job == nil {
		return "", false
	}
	frontendURL := strings.TrimRight(strings.TrimSpace(config.Data.TelegramNotification.FrontendURL), "/")
	sourceURL := strings.TrimSpace(job.SourceURL)
	switch event {
	case socialMediaAnalysisNotificationConflict:
		if frontendURL == "" || strings.TrimSpace(job.JobID) == "" {
			return fmt.Sprintf("⚠️ RoroadMap needs your review for this post:\n%s", sourceURL), true
		}
		return fmt.Sprintf("⚠️ RoroadMap needs your review for this post:\n%s\nReview it here:\n%s/pending-social-analysis?job_id=%s", sourceURL, frontendURL, url.QueryEscape(job.JobID)), true
	case socialMediaAnalysisNotificationCreated:
		if job.CreatedMarkerID == nil {
			return fmt.Sprintf("📍 RoroadMap created a marker from:\n%s", sourceURL), true
		}
		if frontendURL == "" {
			return fmt.Sprintf("📍 RoroadMap created a marker from:\n%s", sourceURL), true
		}
		return fmt.Sprintf("📍 RoroadMap created a marker from:\n%s\nOpen it here:\n%s/markers/%d", sourceURL, frontendURL, *job.CreatedMarkerID), true
	case socialMediaAnalysisNotificationFailed:
		failure := strings.TrimSpace(job.ErrorCode)
		if failure == "" {
			failure = "unknown error"
		}
		return fmt.Sprintf("❌ RoroadMap could not analyze this post (%s):\n%s", failure, sourceURL), true
	default:
		return "", false
	}
}
