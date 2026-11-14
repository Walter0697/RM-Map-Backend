package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"time"
)

type PendingSocialMediaAnalysisItem struct {
	JobID     string          `json:"job_id"`
	SourceURL string          `json:"source_url"`
	Status    string          `json:"status"`
	Stage     string          `json:"stage"`
	CreatedAt string          `json:"created_at"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type PendingSocialMediaAnalysisListResponse struct {
	Count int                              `json:"count"`
	Items []PendingSocialMediaAnalysisItem `json:"items"`
}

func ListPendingSocialMediaAnalysis(ctx context.Context, relationID uint) ([]PendingSocialMediaAnalysisItem, error) {
	if database.Connection == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	var jobs []dbmodel.SocialMediaAnalysisJob
	if err := database.Connection.WithContext(ctx).
		Where("relation_id = ? AND status IN ?", relationID, []string{
			dbmodel.SocialMediaAnalysisJobStatusNeedsInput,
			dbmodel.SocialMediaAnalysisJobStatusAwaitingConfirmation,
		}).
		Order("created_at DESC").
		Find(&jobs).Error; err != nil {
		return nil, err
	}
	items := make([]PendingSocialMediaAnalysisItem, 0, len(jobs))
	for _, job := range jobs {
		item := PendingSocialMediaAnalysisItem{
			JobID:     job.JobID,
			SourceURL: job.SourceURL,
			Status:    job.Status,
			Stage:     job.Stage,
			CreatedAt: job.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		}
		if job.ResultJSON != "" {
			item.Result = json.RawMessage(job.ResultJSON)
		}
		items = append(items, item)
	}
	return items, nil
}

func ApprovePendingSocialMediaAnalysis(ctx context.Context, jobID string, relationID uint) (*dbmodel.SocialMediaAnalysisJob, error) {
	job, err := GetSocialMediaAnalysis(ctx, jobID, relationID)
	if err != nil {
		return nil, err
	}
	if job.Status != dbmodel.SocialMediaAnalysisJobStatusNeedsInput && job.Status != dbmodel.SocialMediaAnalysisJobStatusAwaitingConfirmation {
		return nil, fmt.Errorf("analysis job is not pending review")
	}
	var result AgentAnalysisResult
	if err := json.Unmarshal([]byte(job.ResultJSON), &result); err != nil {
		return nil, fmt.Errorf("decode analysis result: %w", err)
	}
	marker, err := createSocialMediaAnalysisMarker(job, result)
	if err != nil {
		return nil, err
	}
	job.CreatedMarkerID = &marker.ID
	job.Status = dbmodel.SocialMediaAnalysisJobStatusCompleted
	job.Stage = "marker_created"
	now := time.Now().UTC()
	job.CompletedAt = &now
	if err := database.Connection.WithContext(ctx).Save(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

func RejectPendingSocialMediaAnalysis(ctx context.Context, jobID string, relationID uint) (*dbmodel.SocialMediaAnalysisJob, error) {
	job, err := GetSocialMediaAnalysis(ctx, jobID, relationID)
	if err != nil {
		return nil, err
	}
	if job.Status != dbmodel.SocialMediaAnalysisJobStatusNeedsInput && job.Status != dbmodel.SocialMediaAnalysisJobStatusAwaitingConfirmation {
		return nil, fmt.Errorf("analysis job is not pending review")
	}
	job.Status = dbmodel.SocialMediaAnalysisJobStatusCancelled
	job.Stage = "rejected"
	now := time.Now().UTC()
	job.CompletedAt = &now
	if err := database.Connection.WithContext(ctx).Save(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}
