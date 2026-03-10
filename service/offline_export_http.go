package service

import (
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
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
	offlineExportJobsMu          sync.RWMutex
	offlineExportJobs            = map[string]offlineExportJobContract{}
	offlineExportArtifacts       = map[string]map[string]offlineRenderedArtifact{}
	offlineExportSeq             uint64
	offlineExportLoadRecordsFn   = loadOfflineExportRecords
	offlineExportBuildSnapshotFn = BuildExportSnapshot
	offlineExportRunJobFn        = runOfflineExportJob
	offlineExportJobCleanupFn    = cleanupExpiredOfflineExportJobs

	offlineExportMetricJobsCreated   uint64
	offlineExportMetricJobsSucceeded uint64
	offlineExportMetricJobsFailed    uint64
	offlineExportMetricJobsPartial   uint64
)

func IntegrationCreateOfflineExportHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.exports.create", constant.APIKeyScopeSchedulesWrite, "")
	if !ok {
		return
	}

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

	markers, schedules, err := offlineExportLoadRecordsFn(apiKey.Relation.ID, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	snapshot, err := offlineExportBuildSnapshotFn(BuildExportSnapshotInput{
		RelationID: apiKey.Relation.ID,
		UserID:     apiKey.ActorUser.ID,
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
			DownloadURL: fmt.Sprintf("/integration/exports/%s/artifacts/%s", jobID, format),
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

func IntegrationGetOfflineExportStatusHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := integrationAuthenticateRequestFn(w, r, "integration.exports.status", constant.APIKeyScopeSchedulesRead, ""); !ok {
		return
	}

	jobID := strings.TrimSpace(chi.URLParam(r, "job_id"))
	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}

	offlineExportJobsMu.RLock()
	job, exists := offlineExportJobs[jobID]
	offlineExportJobsMu.RUnlock()
	if !exists {
		http.Error(w, "export job not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"job": job})
}

func IntegrationGetOfflineExportArtifactHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := integrationAuthenticateRequestFn(w, r, "integration.exports.artifact", constant.APIKeyScopeSchedulesRead, ""); !ok {
		return
	}

	jobID := strings.TrimSpace(chi.URLParam(r, "job_id"))
	format := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "format")))
	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}
	if _, err := normalizeExportFormats([]string{format}); err != nil {
		http.Error(w, "invalid artifact format", http.StatusBadRequest)
		return
	}

	offlineExportJobsMu.RLock()
	job, exists := offlineExportJobs[jobID]
	offlineExportJobsMu.RUnlock()
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
	output := make([]string, 0, len(formats))
	seen := map[string]bool{}
	for _, format := range formats {
		normalized := strings.ToLower(strings.TrimSpace(format))
		switch normalized {
		case offlineExportFormatText, offlineExportFormatImage, offlineExportFormatNotion:
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

func loadOfflineExportRecords(relationID uint, filters offlineExportFilters) ([]dbmodel.Marker, []dbmodel.Schedule, error) {
	markerQuery := database.Connection.Model(&dbmodel.Marker{}).
		Where("relation_id = ?", relationID).
		Preload("RestaurantInfo")
	scheduleQuery := database.Connection.Model(&dbmodel.Schedule{}).
		Where("relation_id = ?", relationID).
		Preload("SelectedMarker.RestaurantInfo")

	if len(filters.MarkerIDs) > 0 {
		markerQuery = markerQuery.Where("id in ?", filters.MarkerIDs)
	}
	if len(filters.ScheduleIDs) > 0 {
		scheduleQuery = scheduleQuery.Where("id in ?", filters.ScheduleIDs)
	}
	if filters.From != nil {
		scheduleQuery = scheduleQuery.Where("selected_date >= ?", filters.From.UTC())
	}
	if filters.To != nil {
		scheduleQuery = scheduleQuery.Where("selected_date <= ?", filters.To.UTC())
	}

	markers := make([]dbmodel.Marker, 0)
	if err := markerQuery.Find(&markers).Error; err != nil {
		return nil, nil, err
	}
	schedules := make([]dbmodel.Schedule, 0)
	if err := scheduleQuery.Find(&schedules).Error; err != nil {
		return nil, nil, err
	}
	return markers, schedules, nil
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
