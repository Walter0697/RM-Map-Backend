package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const socialMediaAnalysisWorkerInterval = 15 * time.Second

var socialMediaRunnerClientFn = func() *wcodexRunnerClient {
	runnerURL := strings.TrimSpace(config.Data.SocialMediaAnalysis.RunnerURL)
	if runnerURL == "" {
		runnerURL = strings.TrimSpace(os.Getenv("WCODEX_RUNNER_URL"))
	}
	return NewWcodexRunnerClient(runnerURL, nil)
}

func StartSocialMediaAnalysisWorker() {
	if !config.Data.SocialMediaAnalysis.Enable && strings.TrimSpace(os.Getenv("WCODEX_RUNNER_URL")) == "" {
		log.Printf("[social-media-analysis] worker_disabled=true")
		return
	}
	go func() {
		ticker := time.NewTicker(socialMediaAnalysisWorkerInterval)
		defer ticker.Stop()
		for {
			if err := ReconcileSocialMediaAnalysisJobs(context.Background(), 5); err != nil {
				log.Printf("[social-media-analysis] worker_error=%v", err)
			}
			<-ticker.C
		}
	}()
}

func ReconcileSocialMediaAnalysisJobs(ctx context.Context, limit int) error {
	if database.Connection == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	if limit <= 0 {
		limit = 5
	}
	jobs := make([]dbmodel.SocialMediaAnalysisJob, 0, limit)
	transactionErr := database.Connection.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", dbmodel.SocialMediaAnalysisJobStatusQueued, time.Now().UTC()).Order("id asc").Limit(limit).Find(&jobs).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for idx := range jobs {
			jobs[idx].Status = dbmodel.SocialMediaAnalysisJobStatusProcessing
			jobs[idx].Stage = "processing"
			jobs[idx].Attempts++
			jobs[idx].StartedAt = &now
			if err := tx.Save(&jobs[idx]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if transactionErr != nil {
		return transactionErr
	}
	for idx := range jobs {
		if err := ProcessSocialMediaAnalysisJob(ctx, &jobs[idx]); err != nil {
			log.Printf("[social-media-analysis] job_id=%s process_error=%v", jobs[idx].JobID, err)
		}
	}
	return nil
}

func ProcessSocialMediaAnalysisJob(ctx context.Context, job *dbmodel.SocialMediaAnalysisJob) error {
	if job == nil {
		return fmt.Errorf("analysis job is required")
	}
	if database.Connection == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	if strings.TrimSpace(job.RunnerJobID) != "" {
		return reconcileRunnerJob(ctx, job)
	}
	parsedURL, err := ValidatePublicSourceURL(job.SourceURL)
	if err != nil {
		return markSocialMediaAnalysisFailed(job, "invalid_source_url", err)
	}
	adapter := SelectSourceAdapter(parsedURL.String())
	post, err := adapter.Fetch(ctx, parsedURL)
	if err != nil {
		return markSocialMediaAnalysisFailed(job, "source_fetch_failed", err)
	}
	job.Stage = "source_fetched"
	if err := database.Connection.Save(job).Error; err != nil {
		return err
	}
	client := socialMediaRunnerClientFn()
	prompt := buildSocialMediaAnalysisPrompt(post)
	runnerJob, err := client.Submit(ctx, RunnerSubmitRequest{
		Prompt:       prompt,
		Application:  "roroadmap",
		Operation:    "social-media-analysis",
		Timeout:      socialMediaRunnerTimeoutSeconds(),
		OutputSchema: socialMediaAnalysisOutputSchema(),
	})
	if err != nil {
		return markSocialMediaAnalysisFailed(job, "runner_submit_failed", err)
	}
	job.RunnerJobID = runnerJob.ID
	job.Stage = "agent_submitted"
	return database.Connection.Save(job).Error
}

func socialMediaRunnerTimeoutSeconds() int {
	if config.Data.SocialMediaAnalysis.RunnerTimeoutSeconds > 0 {
		return config.Data.SocialMediaAnalysis.RunnerTimeoutSeconds
	}
	return 600
}

func reconcileRunnerJob(ctx context.Context, job *dbmodel.SocialMediaAnalysisJob) error {
	runnerJob, err := socialMediaRunnerClientFn().Get(ctx, job.RunnerJobID)
	if err != nil {
		return err
	}
	switch runnerJob.Status {
	case RunnerJobStatusAccepted, RunnerJobStatusQueued, RunnerJobStatusRunning:
		job.Stage = "agent_running"
		return database.Connection.Save(job).Error
	case RunnerJobStatusFailed, RunnerJobStatusCancelled, RunnerJobStatusTimedOut:
		return markSocialMediaAnalysisFailed(job, "runner_"+runnerJob.Status, fmt.Errorf("runner job ended with status %s: %s", runnerJob.Status, runnerJob.Error))
	case RunnerJobStatusDone:
		var result AgentAnalysisResult
		if err := json.Unmarshal(runnerJob.Result, &result); err != nil {
			return markSocialMediaAnalysisFailed(job, "invalid_agent_result", err)
		}
		if err := ApplyAgentAnalysisResult(job, result); err != nil {
			return markSocialMediaAnalysisFailed(job, "invalid_agent_result", err)
		}
		now := time.Now().UTC()
		job.CompletedAt = &now
		return database.Connection.Save(job).Error
	default:
		return fmt.Errorf("unknown runner status %q", runnerJob.Status)
	}
}

func markSocialMediaAnalysisFailed(job *dbmodel.SocialMediaAnalysisJob, code string, err error) error {
	job.Status = dbmodel.SocialMediaAnalysisJobStatusFailed
	job.Stage = "failed"
	job.ErrorCode = code
	job.ErrorDetail = strings.TrimSpace(err.Error())
	now := time.Now().UTC()
	job.CompletedAt = &now
	return database.Connection.Save(job).Error
}

func buildSocialMediaAnalysisPrompt(post NormalizedSocialPost) string {
	rawBody := post.RawBody
	if len(rawBody) > 32000 {
		rawBody = rawBody[:32000]
	}
	return fmt.Sprintf("Analyze this public social-media source and return only the required JSON output. URL: %s\nFinal URL: %s\nFetched content:\n%s", post.SourceURL, post.FinalURL, rawBody)
}

func socialMediaAnalysisOutputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"recommendation": map[string]any{"type": "string", "enum": []string{AgentRecommendationCompleted, AgentRecommendationNeedsInput, AgentRecommendationAwaitingConfirmation, AgentRecommendationCreateMarker}},
			"description":    map[string]any{"type": "string"},
			"confidence":     map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"follow_up":      map[string]any{"type": "string"},
			"evidence":       map[string]any{"type": "array"},
		},
		"required": []string{"recommendation", "confidence"},
	}
}
