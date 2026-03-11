package service

import (
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi"
)

const (
	offlineExportFormatText   = "text"
	offlineExportFormatImage  = "image"
	offlineExportFormatNotion = "notion"

	offlineExportJobStatusQueued     = "queued"
	offlineExportJobStatusProcessing = "processing"
	offlineExportJobStatusSucceeded  = "succeeded"
	offlineExportJobStatusFailed     = "failed"
	offlineExportJobStatusPartial    = "partial_success"

	offlineExportJobRetention = 24 * time.Hour
)

type offlineExportCreateRequest struct {
	MarkerIDs    []int    `json:"marker_ids"`
	ScheduleIDs  []int    `json:"schedule_ids"`
	ScheduleFrom string   `json:"schedule_from"`
	ScheduleTo   string   `json:"schedule_to"`
	Timezone     string   `json:"timezone"`
	Formats      []string `json:"formats"`
}

type offlineExportArtifactContract struct {
	Format      string `json:"format"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url"`
}

type offlineExportJobContract struct {
	JobID     string                          `json:"job_id"`
	Status    string                          `json:"status"`
	CreatedAt time.Time                       `json:"created_at"`
	UpdatedAt time.Time                       `json:"updated_at"`
	Snapshot  *ExportSnapshot                 `json:"snapshot,omitempty"`
	Artifacts []offlineExportArtifactContract `json:"artifacts"`
}

type offlineExportCreateResponse struct {
	Job offlineExportJobContract `json:"job"`
}

var (
	offlineExportJobsMu            sync.RWMutex
	offlineExportJobs              = map[string]offlineExportJobContract{}
	offlineExportArtifacts         = map[string]map[string]offlineRenderedArtifact{}
	offlineExportSeq               uint64
	offlineExportLoadRecordsFn     = loadOfflineExportRecords
	offlineExportBuildSnapshotFn   = BuildExportSnapshot
	offlineExportRunJobFn          = runOfflineExportJob
	offlineExportJobCleanupFn      = cleanupExpiredOfflineExportJobs
	offlineExportCurrentUserFn     = currentUserFromRequest
	offlineExportCurrentRelationFn = GetCurrentRelation

	offlineExportMetricJobsCreated   uint64
	offlineExportMetricJobsSucceeded uint64
	offlineExportMetricJobsFailed    uint64
	offlineExportMetricJobsPartial   uint64
)

func IntegrationCreateOfflineExportHandler(w http.ResponseWriter, r *http.Request) {
	if !offlineExportFeatureEnabled() {
		http.Error(w, "offline export feature is disabled", http.StatusNotFound)
		return
	}

	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.exports.create", constant.APIKeyScopeSchedulesWrite, "")
	if !ok {
		return
	}

	createOfflineExportJobAndRespond(w, r, apiKey.Relation.ID, apiKey.ActorUser.ID, "/integration/exports")
}

func CreateOfflineExportHandler(w http.ResponseWriter, r *http.Request) {
	if !offlineExportFeatureEnabled() {
		http.Error(w, "offline export feature is disabled", http.StatusNotFound)
		return
	}

	user := offlineExportCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	relation, err := offlineExportCurrentRelationFn(*user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if relation == nil {
		http.Error(w, "selected relation is required", http.StatusBadRequest)
		return
	}

	createOfflineExportJobAndRespond(w, r, relation.ID, user.ID, "/exports")
}

func IntegrationGetOfflineExportStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !offlineExportFeatureEnabled() {
		http.Error(w, "offline export feature is disabled", http.StatusNotFound)
		return
	}

	if _, ok := integrationAuthenticateRequestFn(w, r, "integration.exports.status", constant.APIKeyScopeSchedulesRead, ""); !ok {
		return
	}

	writeOfflineExportStatusResponse(w, chi.URLParam(r, "job_id"))
}

func GetOfflineExportStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !offlineExportFeatureEnabled() {
		http.Error(w, "offline export feature is disabled", http.StatusNotFound)
		return
	}

	user := offlineExportCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	relation, err := offlineExportCurrentRelationFn(*user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if relation == nil {
		http.Error(w, "selected relation is required", http.StatusBadRequest)
		return
	}

	jobID := chi.URLParam(r, "job_id")
	job, exists := getOfflineExportJob(strings.TrimSpace(jobID))
	if !exists {
		http.Error(w, "export job not found", http.StatusNotFound)
		return
	}
	if !offlineExportJobBelongsToRelation(job, relation.ID) {
		http.Error(w, "export job not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"job": job})
}

func IntegrationGetOfflineExportArtifactHandler(w http.ResponseWriter, r *http.Request) {
	if !offlineExportFeatureEnabled() {
		http.Error(w, "offline export feature is disabled", http.StatusNotFound)
		return
	}

	if _, ok := integrationAuthenticateRequestFn(w, r, "integration.exports.artifact", constant.APIKeyScopeSchedulesRead, ""); !ok {
		return
	}

	writeOfflineExportArtifactResponse(w, chi.URLParam(r, "job_id"), chi.URLParam(r, "format"))
}

func GetOfflineExportArtifactHandler(w http.ResponseWriter, r *http.Request) {
	if !offlineExportFeatureEnabled() {
		http.Error(w, "offline export feature is disabled", http.StatusNotFound)
		return
	}

	user := offlineExportCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	relation, err := offlineExportCurrentRelationFn(*user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if relation == nil {
		http.Error(w, "selected relation is required", http.StatusBadRequest)
		return
	}

	jobID := strings.TrimSpace(chi.URLParam(r, "job_id"))
	job, exists := getOfflineExportJob(jobID)
	if !exists {
		http.Error(w, "export job not found", http.StatusNotFound)
		return
	}
	if !offlineExportJobBelongsToRelation(job, relation.ID) {
		http.Error(w, "export job not found", http.StatusNotFound)
		return
	}

	writeOfflineExportArtifactResponse(w, jobID, chi.URLParam(r, "format"))
}

func createOfflineExportJobAndRespond(w http.ResponseWriter, r *http.Request, relationID uint, userID uint, downloadBasePath string) {

	request := offlineExportCreateRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	formats, err := normalizeExportFormats(request.Formats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filters, err := normalizeOfflineExportFilters(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	markers, schedules, err := offlineExportLoadRecordsFn(relationID, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	snapshot, err := offlineExportBuildSnapshotFn(BuildExportSnapshotInput{
		RelationID: relationID,
		UserID:     userID,
		Timezone:   filters.Timezone,
		Now:        time.Now().UTC(),
		Markers:    markers,
		Schedules:  schedules,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobID := fmt.Sprintf("export_%d", atomic.AddUint64(&offlineExportSeq, 1))
	now := time.Now().UTC()
	artifacts := make([]offlineExportArtifactContract, 0, len(formats))
	for _, format := range formats {
		artifacts = append(artifacts, offlineExportArtifactContract{
			Format:      format,
			Status:      offlineExportJobStatusQueued,
			DownloadURL: fmt.Sprintf("%s/%s/artifacts/%s", strings.TrimRight(downloadBasePath, "/"), jobID, format),
		})
	}

	job := offlineExportJobContract{
		JobID:     jobID,
		Status:    offlineExportJobStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
		Snapshot:  snapshot,
		Artifacts: artifacts,
	}

	offlineExportJobsMu.Lock()
	offlineExportJobCleanupFn(now)
	offlineExportJobs[jobID] = job
	offlineExportArtifacts[jobID] = map[string]offlineRenderedArtifact{}
	offlineExportJobsMu.Unlock()
	atomic.AddUint64(&offlineExportMetricJobsCreated, 1)
	log.Printf("[offline-export] event=job_created job_id=%s formats=%d markers=%d schedules=%d", jobID, len(formats), len(markers), len(schedules))
	go offlineExportRunJobFn(jobID)

	respondJSON(w, http.StatusAccepted, offlineExportCreateResponse{Job: job})
}

type offlineExportFilters struct {
	Timezone    string
	MarkerIDs   []uint
	ScheduleIDs []uint
	From        *time.Time
	To          *time.Time
}

func normalizeOfflineExportFilters(request offlineExportCreateRequest) (offlineExportFilters, error) {
	filters := offlineExportFilters{
		Timezone: strings.TrimSpace(request.Timezone),
	}
	if filters.Timezone == "" {
		filters.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(filters.Timezone); err != nil {
		return filters, fmt.Errorf("invalid timezone")
	}

	for _, id := range request.MarkerIDs {
		if id <= 0 {
			return filters, fmt.Errorf("marker_ids must contain positive integers")
		}
		filters.MarkerIDs = append(filters.MarkerIDs, uint(id))
	}
	for _, id := range request.ScheduleIDs {
		if id <= 0 {
			return filters, fmt.Errorf("schedule_ids must contain positive integers")
		}
		filters.ScheduleIDs = append(filters.ScheduleIDs, uint(id))
	}

	if strings.TrimSpace(request.ScheduleFrom) != "" {
		from, err := time.Parse(time.RFC3339, strings.TrimSpace(request.ScheduleFrom))
		if err != nil {
			return filters, fmt.Errorf("schedule_from must be RFC3339")
		}
		filters.From = &from
	}
	if strings.TrimSpace(request.ScheduleTo) != "" {
		to, err := time.Parse(time.RFC3339, strings.TrimSpace(request.ScheduleTo))
		if err != nil {
			return filters, fmt.Errorf("schedule_to must be RFC3339")
		}
		filters.To = &to
	}
	if filters.From != nil && filters.To != nil && filters.To.Before(*filters.From) {
		return filters, fmt.Errorf("schedule_to must be greater than or equal to schedule_from")
	}

	return filters, nil
}

func normalizeExportFormats(formats []string) ([]string, error) {
	if len(formats) == 0 {
		return nil, fmt.Errorf("formats are required")
	}
	allowedFormats := offlineExportAllowedFormats()
	output := make([]string, 0, len(formats))
	seen := map[string]bool{}
	for _, format := range formats {
		normalized := strings.ToLower(strings.TrimSpace(format))
		switch normalized {
		case offlineExportFormatText, offlineExportFormatImage, offlineExportFormatNotion:
			if !allowedFormats[normalized] {
				return nil, fmt.Errorf("format is disabled: %s", normalized)
			}
			if !seen[normalized] {
				seen[normalized] = true
				output = append(output, normalized)
			}
		default:
			return nil, fmt.Errorf("unsupported format: %s", normalized)
		}
	}
	if len(output) == 0 {
		return nil, fmt.Errorf("formats are required")
	}
	return output, nil
}

func offlineExportFeatureEnabled() bool {
	value, exists := os.LookupEnv("OFFLINE_EXPORT_ENABLE")
	if !exists {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func offlineExportAllowedFormats() map[string]bool {
	if !offlineExportFeatureEnabled() {
		return map[string]bool{}
	}

	rawValue := strings.TrimSpace(os.Getenv("OFFLINE_EXPORT_FORMATS"))
	if rawValue == "" {
		return map[string]bool{
			offlineExportFormatText:   true,
			offlineExportFormatImage:  true,
			offlineExportFormatNotion: true,
		}
	}

	allowed := map[string]bool{}
	for _, token := range strings.Split(rawValue, ",") {
		switch strings.ToLower(strings.TrimSpace(token)) {
		case offlineExportFormatText, offlineExportFormatImage, offlineExportFormatNotion:
			allowed[strings.ToLower(strings.TrimSpace(token))] = true
		}
	}
	if len(allowed) == 0 {
		allowed[offlineExportFormatText] = true
		allowed[offlineExportFormatImage] = true
		allowed[offlineExportFormatNotion] = true
	}
	return allowed
}

func OfflineExportAllowedFormatsList() []string {
	allowed := offlineExportAllowedFormats()
	output := make([]string, 0, len(allowed))
	for _, format := range []string{
		offlineExportFormatText,
		offlineExportFormatImage,
		offlineExportFormatNotion,
	} {
		if allowed[format] {
			output = append(output, format)
		}
	}
	return output
}

func loadOfflineExportRecords(relationID uint, filters offlineExportFilters) ([]dbmodel.Marker, []dbmodel.Schedule, error) {
	scheduleQuery := database.Connection.Model(&dbmodel.Schedule{}).
		Where("relation_id = ?", relationID).
		Preload("SelectedMarker.RestaurantInfo")

	if len(filters.ScheduleIDs) > 0 {
		scheduleQuery = scheduleQuery.Where("id in ?", filters.ScheduleIDs)
	}
	if filters.From != nil {
		scheduleQuery = scheduleQuery.Where("selected_date >= ?", filters.From.UTC())
	}
	if filters.To != nil {
		scheduleQuery = scheduleQuery.Where("selected_date <= ?", filters.To.UTC())
	}

	schedules := make([]dbmodel.Schedule, 0)
	if err := scheduleQuery.Find(&schedules).Error; err != nil {
		return nil, nil, err
	}

	markerIDs := make([]uint, 0, len(filters.MarkerIDs))
	markerIDSeen := map[uint]bool{}
	for _, markerID := range filters.MarkerIDs {
		if markerID == 0 || markerIDSeen[markerID] {
			continue
		}
		markerIDSeen[markerID] = true
		markerIDs = append(markerIDs, markerID)
	}
	if len(markerIDs) == 0 {
		for _, schedule := range schedules {
			if schedule.MarkerId == nil || *schedule.MarkerId == 0 || markerIDSeen[*schedule.MarkerId] {
				continue
			}
			markerIDSeen[*schedule.MarkerId] = true
			markerIDs = append(markerIDs, *schedule.MarkerId)
		}
	}

	markers := make([]dbmodel.Marker, 0, len(markerIDs))
	if len(markerIDs) == 0 {
		return markers, schedules, nil
	}
	markerQuery := database.Connection.Model(&dbmodel.Marker{}).
		Where("relation_id = ?", relationID).
		Where("id in ?", markerIDs).
		Preload("RestaurantInfo")
	if err := markerQuery.Find(&markers).Error; err != nil {
		return nil, nil, err
	}
	return markers, schedules, nil
}

func writeOfflineExportStatusResponse(w http.ResponseWriter, rawJobID string) {
	jobID := strings.TrimSpace(rawJobID)
	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}

	job, exists := getOfflineExportJob(jobID)
	if !exists {
		http.Error(w, "export job not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"job": job})
}

func writeOfflineExportArtifactResponse(w http.ResponseWriter, rawJobID string, rawFormat string) {
	jobID := strings.TrimSpace(rawJobID)
	format := strings.ToLower(strings.TrimSpace(rawFormat))
	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}
	if _, err := normalizeExportFormats([]string{format}); err != nil {
		http.Error(w, "invalid artifact format", http.StatusBadRequest)
		return
	}

	job, exists := getOfflineExportJob(jobID)
	if !exists {
		http.Error(w, "export job not found", http.StatusNotFound)
		return
	}

	for _, artifact := range job.Artifacts {
		if artifact.Format == format {
			if artifact.Status != offlineExportJobStatusSucceeded {
				http.Error(w, "artifact is not ready", http.StatusConflict)
				return
			}
			offlineExportJobsMu.RLock()
			jobArtifacts := offlineExportArtifacts[jobID]
			rendered, ok := jobArtifacts[format]
			offlineExportJobsMu.RUnlock()
			if !ok || len(rendered.Content) == 0 {
				http.Error(w, "artifact payload unavailable", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", rendered.ContentType)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", rendered.FileName))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(rendered.Content)
			return
		}
	}

	http.Error(w, "artifact format not found for export job", http.StatusNotFound)
}

func getOfflineExportJob(jobID string) (offlineExportJobContract, bool) {
	offlineExportJobsMu.RLock()
	job, exists := offlineExportJobs[jobID]
	offlineExportJobsMu.RUnlock()
	return job, exists
}

func offlineExportJobBelongsToRelation(job offlineExportJobContract, relationID uint) bool {
	if job.Snapshot == nil {
		return false
	}
	return job.Snapshot.Source.RelationID == relationID
}

func runOfflineExportJob(jobID string) {
	jobStartedAt := time.Now().UTC()
	offlineExportJobsMu.Lock()
	job, exists := offlineExportJobs[jobID]
	if !exists {
		offlineExportJobsMu.Unlock()
		return
	}
	job.Status = offlineExportJobStatusProcessing
	job.UpdatedAt = time.Now().UTC()
	offlineExportJobs[jobID] = job
	offlineExportJobsMu.Unlock()

	hasFailure := false
	hasSuccess := false

	for index, artifact := range job.Artifacts {
		formatStartedAt := time.Now().UTC()
		offlineExportJobsMu.Lock()
		current := offlineExportJobs[jobID]
		if index < len(current.Artifacts) {
			current.Artifacts[index].Status = offlineExportJobStatusProcessing
			current.UpdatedAt = time.Now().UTC()
			offlineExportJobs[jobID] = current
		}
		offlineExportJobsMu.Unlock()

		time.Sleep(10 * time.Millisecond)

		nextStatus := offlineExportJobStatusSucceeded
		errorCategory := ""
		renderedArtifact, renderErr := renderOfflineExportArtifact(artifact.Format, job.Snapshot, jobID)
		if renderErr != nil {
			nextStatus = offlineExportJobStatusFailed
			errorCategory = "render_error"
		}
		if nextStatus == offlineExportJobStatusFailed {
			hasFailure = true
		} else {
			hasSuccess = true
		}

		offlineExportJobsMu.Lock()
		current = offlineExportJobs[jobID]
		if index < len(current.Artifacts) {
			current.Artifacts[index].Status = nextStatus
			current.UpdatedAt = time.Now().UTC()
			offlineExportJobs[jobID] = current
			if nextStatus == offlineExportJobStatusSucceeded {
				if _, ok := offlineExportArtifacts[jobID]; !ok {
					offlineExportArtifacts[jobID] = map[string]offlineRenderedArtifact{}
				}
				offlineExportArtifacts[jobID][artifact.Format] = renderedArtifact
			}
		}
		offlineExportJobsMu.Unlock()
		log.Printf("[offline-export] event=format_completed job_id=%s format=%s status=%s duration_ms=%d error_category=%s",
			jobID,
			artifact.Format,
			nextStatus,
			time.Since(formatStartedAt).Milliseconds(),
			errorCategory,
		)
	}

	offlineExportJobsMu.Lock()
	current := offlineExportJobs[jobID]
	switch {
	case hasSuccess && hasFailure:
		current.Status = offlineExportJobStatusPartial
		atomic.AddUint64(&offlineExportMetricJobsPartial, 1)
	case hasFailure:
		current.Status = offlineExportJobStatusFailed
		atomic.AddUint64(&offlineExportMetricJobsFailed, 1)
	default:
		current.Status = offlineExportJobStatusSucceeded
		atomic.AddUint64(&offlineExportMetricJobsSucceeded, 1)
	}
	current.UpdatedAt = time.Now().UTC()
	offlineExportJobCleanupFn(current.UpdatedAt)
	offlineExportJobs[jobID] = current
	offlineExportJobsMu.Unlock()
	log.Printf("[offline-export] event=job_completed job_id=%s status=%s duration_ms=%d metrics_created=%d metrics_succeeded=%d metrics_failed=%d metrics_partial=%d",
		jobID,
		current.Status,
		time.Since(jobStartedAt).Milliseconds(),
		atomic.LoadUint64(&offlineExportMetricJobsCreated),
		atomic.LoadUint64(&offlineExportMetricJobsSucceeded),
		atomic.LoadUint64(&offlineExportMetricJobsFailed),
		atomic.LoadUint64(&offlineExportMetricJobsPartial),
	)
}

func isSupportedExportFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case offlineExportFormatText, offlineExportFormatImage, offlineExportFormatNotion:
		return true
	default:
		return false
	}
}

func cleanupExpiredOfflineExportJobs(now time.Time) {
	expirationCutoff := now.Add(-offlineExportJobRetention)
	for jobID, job := range offlineExportJobs {
		if job.UpdatedAt.After(expirationCutoff) {
			continue
		}
		if !isOfflineExportTerminalStatus(job.Status) {
			continue
		}
		delete(offlineExportJobs, jobID)
		delete(offlineExportArtifacts, jobID)
	}
}

func isOfflineExportTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case offlineExportJobStatusSucceeded, offlineExportJobStatusFailed, offlineExportJobStatusPartial:
		return true
	default:
		return false
	}
}
