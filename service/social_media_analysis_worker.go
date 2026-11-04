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
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const socialMediaAnalysisWorkerInterval = 15 * time.Second
const socialMediaLocationComparisonStage = "location_comparison_submitted"

type socialAnalysisTracerProvider interface {
	Start(context.Context, string, map[string]string) (context.Context, SpanHandle)
}

type socialMediaRunnerClient interface {
	Submit(context.Context, RunnerSubmitRequest) (RunnerJob, error)
	Get(context.Context, string) (RunnerJob, error)
}

var socialMediaRunnerClientFn = func() socialMediaRunnerClient {
	runnerURL := strings.TrimSpace(config.Data.SocialMediaAnalysis.RunnerURL)
	if runnerURL == "" {
		runnerURL = strings.TrimSpace(os.Getenv("WCODEX_RUNNER_URL"))
	}
	return NewWcodexRunnerClient(runnerURL, nil)
}

var socialAnalysisTracerFn = func() socialAnalysisTracerProvider {
	tracer, _ := NewSocialAnalysisTracer(config.Data.Phoenix, NewSecretResolver(config.Data.Infisical, http.DefaultClient))
	return tracer
}

var socialAnalysisAssetStoreFn = func() AssetStore {
	store, err := NewMinIOAssetStore(config.Data.MinIO, NewSecretResolver(config.Data.Infisical, http.DefaultClient), http.DefaultClient)
	if err != nil {
		log.Printf("[observability] MinIO asset store unavailable: %v", err)
		return nil
	}
	return store
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
			if !strings.HasPrefix(jobs[idx].Stage, "location_comparison") {
				jobs[idx].Stage = "processing"
			}
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
	adapter := selectSocialMediaAnalysisSourceAdapter(parsedURL.String())
	tracer := socialAnalysisTracerFn()
	sourceCtx, sourceSpan := tracer.Start(ctx, "source.fetch", map[string]string{
		"analysis.job_id":      job.JobID,
		"analysis.source_kind": adapter.Name(),
		"provider.name":        adapter.Name(),
		"provider.operation":   "fetch",
	})
	post, err := adapter.Fetch(sourceCtx, parsedURL)
	if err != nil {
		sourceSpan.RecordError(err)
		sourceSpan.SetStatus(false, "source fetch failed")
		sourceSpan.End()
		return markSocialMediaAnalysisFailed(job, "source_fetch_failed", err)
	}
	if post.Metadata != nil {
		if runID := strings.TrimSpace(post.Metadata["apify_run_id"]); runID != "" {
			sourceSpan.SetAttribute("provider.run_id", runID)
		}
	}
	sourceSpan.SetStatus(true, "source fetched")
	sourceSpan.End()
	sourceJSON, err := json.Marshal(post)
	if err != nil {
		return markSocialMediaAnalysisFailed(job, "source_normalization_failed", err)
	}
	job.SourceJSON = string(sourceJSON)
	captureSocialAnalysisArtifacts(ctx, job, post, sourceJSON, socialAnalysisAssetStoreFn())
	job.Stage = "source_fetched"
	if err := database.Connection.Save(job).Error; err != nil {
		return err
	}
	allowedMarkerTypes, err := socialMediaAnalysisMarkerTypeValues()
	if err != nil {
		return markSocialMediaAnalysisFailed(job, "marker_types_fetch_failed", err)
	}
	client := socialMediaRunnerClientFn()
	prompt := buildSocialMediaAnalysisPrompt(post, allowedMarkerTypes)
	runnerJob, err := client.Submit(ctx, RunnerSubmitRequest{
		Prompt:       prompt,
		Application:  "roroadmap",
		Operation:    "social-media-analysis",
		Timeout:      socialMediaRunnerTimeoutSeconds(),
		OutputSchema: socialMediaAnalysisOutputSchema(allowedMarkerTypes),
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
		if job.Stage != socialMediaLocationComparisonStage {
			if comparisonResult, submitted, err := maybeSubmitLocationComparison(ctx, job, &result); err != nil {
				return markSocialMediaAnalysisFailed(job, "location_comparison_failed", err)
			} else if submitted {
				job.RunnerJobID = comparisonResult.ID
				job.Stage = socialMediaLocationComparisonStage
				return database.Connection.Save(job).Error
			}
		}
		return finalizeSocialMediaAnalysisJob(job, result)
	default:
		return fmt.Errorf("unknown runner status %q", runnerJob.Status)
	}
}

func finalizeSocialMediaAnalysisJob(job *dbmodel.SocialMediaAnalysisJob, result AgentAnalysisResult) error {
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
}

var googlePlacesProviderFn = func() GooglePlacesProvider {
	if !config.Data.GoogleMaps.Enable {
		return nil
	}
	setting := config.Data.Infisical
	setting.GoogleMapsAPIKey = config.Data.GoogleMaps.APIKey
	provider := NewGooglePlacesProvider(config.Data.GoogleMaps, NewSecretResolver(setting, http.DefaultClient), http.DefaultClient)
	return provider
}

func maybeSubmitLocationComparison(ctx context.Context, job *dbmodel.SocialMediaAnalysisJob, result *AgentAnalysisResult) (RunnerJob, bool, error) {
	if result == nil || result.Recommendation != AgentRecommendationCreateMarker || result.Location == nil {
		return RunnerJob{}, false, nil
	}
	provider := googlePlacesProviderFn()
	if provider == nil {
		return RunnerJob{}, false, nil
	}
	query := locationSearchQuery(*result.Location)
	if query == "" {
		return RunnerJob{}, false, nil
	}
	places, err := provider.Search(ctx, query)
	if err != nil {
		log.Printf("[social-media-analysis] Google Places unavailable, keeping Agent 1 result: %v", err)
		return RunnerJob{}, false, nil
	}
	verification := VerifyLocation(*result.Location, places)
	if verification.Status == LocationMatch && verification.Candidate != nil {
		result.Location.Name = verification.Candidate.Name
		result.Location.Address = verification.Candidate.Address
		result.Location.Latitude = verification.Candidate.Latitude
		result.Location.Longitude = verification.Candidate.Longitude
		return RunnerJob{}, false, nil
	}
	if verification.Status == LocationMatch {
		return RunnerJob{}, false, nil
	}
	prompt := buildLocationComparisonPrompt(job, *result, verification.Candidates)
	allowedTypes := []string{"restaurant"}
	if markerType := strings.TrimSpace(result.MarkerType); markerType != "" {
		allowedTypes = []string{markerType}
	}
	runnerJob, err := socialMediaRunnerClientFn().Submit(ctx, RunnerSubmitRequest{
		Prompt:       prompt,
		Application:  "roroadmap",
		Operation:    "social-media-location-comparison",
		Timeout:      socialMediaRunnerTimeoutSeconds(),
		OutputSchema: socialMediaAnalysisOutputSchema(allowedTypes),
	})
	if err != nil {
		return RunnerJob{}, false, err
	}
	return runnerJob, true, nil
}

func locationSearchQuery(location LocationCandidate) string {
	return strings.TrimSpace(strings.Join([]string{location.Name, location.Address}, ", "))
}

func buildLocationComparisonPrompt(job *dbmodel.SocialMediaAnalysisJob, result AgentAnalysisResult, places []GooglePlaceCandidate) string {
	post := NormalizedSocialPost{}
	if job != nil {
		_ = json.Unmarshal([]byte(job.SourceJSON), &post)
	}
	candidates, _ := json.Marshal(places)
	firstResult, _ := json.Marshal(result)
	return fmt.Sprintf("Review the first agent's proposed location against the Google Places candidates and return only the required JSON output. Use create_marker only when one candidate is clearly the same venue and location. Copy the selected candidate's name, address, latitude, and longitude into location. If candidates are ambiguous or none is reliable, use needs_input and set location to null. Do not invent a location or use web search.\nSource URL: %s\nTitle: %s\nDescription: %s\nFirst agent result: %s\nGoogle Places candidates: %s", post.SourceURL, post.Title, post.Description, firstResult, candidates)
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

func socialMediaAnalysisMarkerTypeValues() ([]string, error) {
	if database.Connection == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	var markerTypes []dbmodel.MarkerType
	if err := database.Connection.Where("hidden IS DISTINCT FROM TRUE").Order("priority asc, id asc").Find(&markerTypes).Error; err != nil {
		return nil, fmt.Errorf("load marker types: %w", err)
	}
	values := make([]string, 0, len(markerTypes))
	for _, markerType := range markerTypes {
		value := strings.TrimSpace(markerType.Value)
		if value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("no visible marker types are configured")
	}
	return values, nil
}

func resolveSocialMediaAnalysisMarkerType(value string, allowed []string) string {
	value = strings.TrimSpace(value)
	for _, candidate := range allowed {
		if strings.EqualFold(value, strings.TrimSpace(candidate)) {
			return strings.TrimSpace(candidate)
		}
	}
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(candidate), "restaurant") {
			return strings.TrimSpace(candidate)
		}
	}
	if len(allowed) > 0 {
		return strings.TrimSpace(allowed[0])
	}
	return "restaurant"
}

func buildSocialMediaAnalysisPrompt(post NormalizedSocialPost, allowedTypes ...[]string) string {
	rawBody := post.RawBody
	if len(rawBody) > 32000 {
		rawBody = rawBody[:32000]
	}
	types := []string{"restaurant"}
	if len(allowedTypes) > 0 && len(allowedTypes[0]) > 0 {
		types = allowedTypes[0]
	}
	return fmt.Sprintf("Analyze this public source and return only the required JSON output. If it clearly identifies a venue or restaurant with a reliable location, use recommendation create_marker and provide the location coordinates. Use completed for informational content without a reliable venue location. Set marker_type to exactly one value from the allowed marker types list; never invent a new type. Set image_url to the page's direct canonical image URL (prefer the supplied image references, og:image, or equivalent) when one is explicitly available; otherwise return an empty string. Do not return the source page URL, tracking URL, or a guessed image path as image_url. Extract tags as short lowercase labels without #, using supplied hashtags as candidates and adding only supported tags. Use empty tags when none are supported. Extract price as exactly free, cheap, middle, or expensive, or an empty string when unknown. Set need_booking only when booking is explicitly indicated. Set estimate_time as short, medium, or long only when supported. Set from_time and to_time only for reliable event times in YYYY-MM-DD HH:mm:ss+00 format; otherwise use null.\nAllowed marker types: %s\nSource URL: %s\nFinal URL: %s\nTitle: %s\nDescription: %s\nHashtags: %s\nImage references: %s\nVideo references: %s\nMetadata: %s\nFetched content:\n%s", strings.Join(types, ", "), post.SourceURL, post.FinalURL, post.Title, post.Description, strings.Join(post.Hashtags, ", "), strings.Join(post.ImageURLs, "\n"), strings.Join(post.VideoURLs, "\n"), formatSocialMediaMetadata(post.Metadata), rawBody)
}

func formatSocialMediaMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	values := make([]string, 0, len(metadata))
	for key, value := range metadata {
		values = append(values, key+"="+value)
	}
	sort.Strings(values)
	return strings.Join(values, "\n")
}

func socialMediaAnalysisOutputSchema(allowedTypes ...[]string) map[string]any {
	types := []string{"restaurant"}
	if len(allowedTypes) > 0 && len(allowedTypes[0]) > 0 {
		types = allowedTypes[0]
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"recommendation": map[string]any{"type": "string", "enum": []string{AgentRecommendationCompleted, AgentRecommendationNeedsInput, AgentRecommendationAwaitingConfirmation, AgentRecommendationCreateMarker}},
			"description":    map[string]any{"type": "string"},
			"image_url":      map[string]any{"type": "string", "description": "Direct canonical source image URL, or an empty string when unavailable."},
			"marker_type":    map[string]any{"type": "string", "enum": types, "description": "Existing marker type value for the marker, never a new value."},
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
		"required": []string{"recommendation", "description", "image_url", "marker_type", "tags", "price", "need_booking", "estimate_time", "from_time", "to_time", "location", "confidence", "follow_up", "evidence"},
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

func createMarkerWithImageCandidates(input model.NewMarker, candidates []string, create func(model.NewMarker) (*dbmodel.Marker, error)) (*dbmodel.Marker, error) {
	var lastErr error
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		input.ImageLink = &candidate
		marker, err := create(input)
		if err == nil {
			return marker, nil
		}
		log.Printf("social media marker image candidate failed candidate=%q err=%v", candidate, err)
		lastErr = err
	}
	input.ImageLink = nil
	marker, err := create(input)
	if err == nil {
		return marker, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, err
}

func createMarkerWithOptionalImage(input model.NewMarker, create func(model.NewMarker) (*dbmodel.Marker, error)) (*dbmodel.Marker, error) {
	candidates := make([]string, 0, 1)
	if input.ImageLink != nil {
		candidates = append(candidates, *input.ImageLink)
	}
	return createMarkerWithImageCandidates(input, candidates, create)
}

func imageCandidatesForSocialMediaMarker(job *dbmodel.SocialMediaAnalysisJob, result AgentAnalysisResult) []string {
	candidates := make([]string, 0, 2)
	if job != nil && strings.TrimSpace(job.SourceJSON) != "" {
		var post NormalizedSocialPost
		if err := json.Unmarshal([]byte(job.SourceJSON), &post); err == nil {
			candidates = append(candidates, post.ImageURLs...)
		}
	}
	if result.ImageURL != "" {
		candidates = append(candidates, result.ImageURL)
	}
	unique := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
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
	imageCandidates := imageCandidatesForSocialMediaMarker(job, result)
	markerInput.ImageLink = nil
	markerInput.Label = label
	markerInput.Latitude = location.Latitude
	markerInput.Longitude = location.Longitude
	markerInput.Address = location.Address
	allowedMarkerTypes, err := socialMediaAnalysisMarkerTypeValues()
	if err != nil {
		return nil, err
	}
	markerInput.Type = resolveSocialMediaAnalysisMarkerType(result.MarkerType, allowedMarkerTypes)
	markerInput.Link = &link
	markerInput.SocialMediaLink = &socialLink
	markerInput.Permanent = &permanent
	return createMarkerWithImageCandidates(markerInput, imageCandidates, func(input model.NewMarker) (*dbmodel.Marker, error) {
		return CreateMarker(input, nil, relation.UserOne, relation, false)
	})
}
