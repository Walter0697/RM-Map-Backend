package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
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
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", []string{dbmodel.SocialMediaAnalysisJobStatusQueued, dbmodel.SocialMediaAnalysisJobStatusProcessing}, time.Now().UTC()).Order("id asc").Limit(limit).Find(&jobs).Error; err != nil {
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
		result, err := decodeAgentAnalysisResult(runnerJob.Result)
		if err != nil {
			return markSocialMediaAnalysisFailed(job, "invalid_agent_result", err)
		}
		if err := ApplyAgentAnalysisResult(job, result); err != nil {
			return markSocialMediaAnalysisFailed(job, "invalid_agent_result", err)
		}
		if result.Recommendation == AgentRecommendationCreateMarker {
			marker, err := createSocialMediaAnalysisMarker(job, result)
			if err != nil {
				return markSocialMediaAnalysisFailed(job, "marker_creation_failed", err)
			}
			job.CreatedMarkerID = &marker.ID
			job.Status = dbmodel.SocialMediaAnalysisJobStatusCompleted
			job.Stage = "marker_created"
		}
		now := time.Now().UTC()
		job.CompletedAt = &now
		return database.Connection.Save(job).Error
	default:
		return fmt.Errorf("unknown runner status %q", runnerJob.Status)
	}
}

func decodeAgentAnalysisResult(raw json.RawMessage) (AgentAnalysisResult, error) {
	var result AgentAnalysisResult
	if len(raw) == 0 {
		return result, fmt.Errorf("agent result is empty")
	}
	if raw[0] == '"' {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return result, err
		}
		raw = json.RawMessage(encoded)
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	return result, nil
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
	return fmt.Sprintf("Analyze this public source and return only the required JSON output. If it clearly identifies a venue or restaurant with a reliable location, use recommendation create_marker and provide the location coordinates. Use completed for informational content without a reliable venue location. Set image_url to the page's direct canonical image URL (prefer og:image or equivalent) when one is explicitly available; otherwise return an empty string. Do not return the source page URL, tracking URL, or a guessed image path as image_url. Extract tags as short lowercase labels without #, and use empty tags when none are supported. Extract price as exactly free, cheap, middle, or expensive, or an empty string when unknown. Set need_booking only when booking is explicitly indicated. Set estimate_time as short, medium, or long only when supported. Set from_time and to_time only for reliable event times in YYYY-MM-DD HH:mm:ss+00 format; otherwise use null. URL: %s\nFinal URL: %s\nFetched content:\n%s", post.SourceURL, post.FinalURL, rawBody)
}

func socialMediaAnalysisOutputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"recommendation": map[string]any{"type": "string", "enum": []string{AgentRecommendationCompleted, AgentRecommendationNeedsInput, AgentRecommendationAwaitingConfirmation, AgentRecommendationCreateMarker}},
			"description":    map[string]any{"type": "string"},
			"image_url":      map[string]any{"type": "string", "description": "Direct canonical source image URL, or an empty string when unavailable."},
			"tags":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"price":          map[string]any{"type": "string", "enum": []string{"", "free", "cheap", "middle", "expensive"}},
			"need_booking":   map[string]any{"type": "boolean"},
			"estimate_time":  map[string]any{"type": "string", "enum": []string{"", "short", "medium", "long"}},
			"from_time": map[string]any{
				"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}},
			},
			"to_time": map[string]any{
				"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}},
			},
			"location": map[string]any{
				"anyOf": []any{
					map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"name":       map[string]any{"type": "string"},
							"address":    map[string]any{"type": "string"},
							"latitude":   map[string]any{"type": "number"},
							"longitude":  map[string]any{"type": "number"},
							"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
						},
						"required": []string{"name", "address", "latitude", "longitude", "confidence"},
					},
					map[string]any{"type": "null"},
				},
			},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"follow_up":  map[string]any{"type": "string"},
			"evidence": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"source": map[string]any{"type": "string"},
						"claim":  map[string]any{"type": "string"},
					},
					"required": []string{"source", "claim"},
				},
			},
		},
		"required": []string{"recommendation", "description", "image_url", "tags", "price", "need_booking", "estimate_time", "from_time", "to_time", "location", "confidence", "follow_up", "evidence"},
	}
}

func buildSocialMediaAnalysisMarkerInput(result AgentAnalysisResult) model.NewMarker {
	imageURL := strings.TrimSpace(result.ImageURL)
	input := model.NewMarker{}
	if imageURL != "" {
		input.ImageLink = &imageURL
	}
	description := strings.TrimSpace(result.Description)
	tags := normalizeSocialMediaAnalysisTags(result.Tags)
	if len(tags) > 0 {
		if description != "" {
			description += "\n\n"
		}
		description += strings.Join(tags, " ")
	}
	input.Description = &description
	if price := strings.TrimSpace(result.Price); price != "" {
		input.Price = &price
	}
	needBooking := result.NeedBooking
	input.NeedBooking = &needBooking
	if estimateTime := strings.TrimSpace(result.EstimateTime); estimateTime != "" {
		input.EstimateTime = &estimateTime
	}
	if fromTime := strings.TrimSpace(valueOrEmpty(result.FromTime)); fromTime != "" {
		input.FromTime = &fromTime
	}
	if toTime := strings.TrimSpace(valueOrEmpty(result.ToTime)); toTime != "" {
		input.ToTime = &toTime
	}
	return input
}

func normalizeSocialMediaAnalysisTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(strings.TrimPrefix(raw, "#"))
		if tag == "" {
			continue
		}
		tag = strings.ToLower(strings.ReplaceAll(tag, " ", "_"))
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, "#"+tag)
	}
	return result
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func createMarkerWithOptionalImage(input model.NewMarker, create func(model.NewMarker) (*dbmodel.Marker, error)) (*dbmodel.Marker, error) {
	marker, err := create(input)
	if err == nil || input.ImageLink == nil {
		return marker, err
	}

	input.ImageLink = nil
	return create(input)
}

func createSocialMediaAnalysisMarker(job *dbmodel.SocialMediaAnalysisJob, result AgentAnalysisResult) (*dbmodel.Marker, error) {
	if job == nil || result.Location == nil {
		return nil, fmt.Errorf("marker location is required")
	}
	location := result.Location
	label := strings.TrimSpace(location.Name)
	if label == "" {
		label = strings.TrimSpace(location.Address)
	}
	if label == "" || strings.TrimSpace(location.Address) == "" {
		return nil, fmt.Errorf("marker name and address are required")
	}
	if err := validateCoordinates(location.Latitude, location.Longitude); err != nil {
		return nil, err
	}
	relation := dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: job.RelationID}}
	if err := relation.GetWithUserById(database.Connection); err != nil {
		return nil, fmt.Errorf("load marker relation: %w", err)
	}
	link := job.SourceURL
	socialLink := job.SourceURL
	permanent := false
	markerInput := buildSocialMediaAnalysisMarkerInput(result)
	markerInput.Label = label
	markerInput.Latitude = location.Latitude
	markerInput.Longitude = location.Longitude
	markerInput.Address = location.Address
	markerInput.Type = "food"
	markerInput.Link = &link
	markerInput.SocialMediaLink = &socialLink
	markerInput.Permanent = &permanent
	return createMarkerWithOptionalImage(markerInput, func(input model.NewMarker) (*dbmodel.Marker, error) {
		return CreateMarker(input, nil, relation.UserOne, relation, false)
	})
}
