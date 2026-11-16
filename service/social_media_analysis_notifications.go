package service

import (
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

func notifySocialMediaAnalysisEvent(job *dbmodel.SocialMediaAnalysisJob, event string) {
	if job == nil || job.CreatorUserID == nil || database.Connection == nil {
		return
	}

	var user dbmodel.User
	if err := database.Connection.First(&user, *job.CreatorUserID).Error; err != nil {
		log.Printf("social_media_analysis notification user_lookup_failed job_id=%s err=%v", job.JobID, err)
		return
	}
	message, ok := buildSocialMediaAnalysisNotification(job, event)
	if !ok {
		return
	}
	if err := NotifyTelegramMessage(user, message); err != nil {
		log.Printf("social_media_analysis notification failed job_id=%s event=%s actor=%s err=%v", job.JobID, event, strings.TrimSpace(user.Username), err)
	}
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
