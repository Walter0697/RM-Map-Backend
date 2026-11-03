package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"sync/atomic"
	"time"
)

type SocialMediaAnalysisActor struct {
	RelationID uint
}

var socialMediaAnalysisSequence uint64

func ApplyAgentAnalysisResult(job *dbmodel.SocialMediaAnalysisJob, result AgentAnalysisResult) error {
	if job == nil {
		return fmt.Errorf("analysis job is required")
	}
	if err := ValidateAgentAnalysisResult(result); err != nil {
		return err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode agent result: %w", err)
	}
	job.ResultJSON = string(encoded)
	job.Stage = "validated"
	switch result.Recommendation {
	case AgentRecommendationCompleted:
		job.Status = dbmodel.SocialMediaAnalysisJobStatusCompleted
	case AgentRecommendationNeedsInput:
		job.Status = dbmodel.SocialMediaAnalysisJobStatusNeedsInput
	case AgentRecommendationAwaitingConfirmation, AgentRecommendationCreateMarker:
		job.Status = dbmodel.SocialMediaAnalysisJobStatusAwaitingConfirmation
	}
	return nil
}

func CreateSocialMediaAnalysis(ctx context.Context, request SocialMediaAnalysisRequest, actor SocialMediaAnalysisActor) (*dbmodel.SocialMediaAnalysisJob, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if actor.RelationID == 0 {
		return nil, fmt.Errorf("relation ID is required")
	}
	parsed, err := ValidatePublicSourceURL(request.URL)
	if err != nil {
		return nil, err
	}
	if database.Connection == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	now := time.Now().UTC()
	job := dbmodel.NewSocialMediaAnalysisJob(parsed.String())
	job.JobID = fmt.Sprintf("analysis_%d_%d", now.UnixNano(), atomic.AddUint64(&socialMediaAnalysisSequence, 1))
	job.RelationID = actor.RelationID
	job.NextAttemptAt = &now
	if err := database.Connection.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("create social media analysis job: %w", err)
	}
	return job, nil
}

func GetSocialMediaAnalysis(ctx context.Context, jobID string, relationID uint) (*dbmodel.SocialMediaAnalysisJob, error) {
	if database.Connection == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || relationID == 0 {
		return nil, fmt.Errorf("job ID and relation ID are required")
	}
	job := &dbmodel.SocialMediaAnalysisJob{}
	if err := database.Connection.WithContext(ctx).Where("job_id = ? AND relation_id = ?", jobID, relationID).First(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

func socialMediaAnalysisResponse(job *dbmodel.SocialMediaAnalysisJob) SocialMediaAnalysisResponse {
	response := SocialMediaAnalysisResponse{
		JobID:  job.JobID,
		Status: job.Status,
		Stage:  job.Stage,
	}
	if strings.TrimSpace(job.ResultJSON) != "" {
		response.Result = json.RawMessage(job.ResultJSON)
	}
	return response
}
