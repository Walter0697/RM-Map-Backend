package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"mapmarker/backend/database/dbmodel"

	"github.com/go-chi/chi"
)

func resetOfflineExportHooks() {
	_ = os.Unsetenv("OFFLINE_EXPORT_ENABLE")
	_ = os.Unsetenv("OFFLINE_EXPORT_FORMATS")
	integrationAuthenticateRequestFn = authenticateIntegrationRequest
	offlineExportLoadRecordsFn = loadOfflineExportRecords
	offlineExportBuildSnapshotFn = BuildExportSnapshot
	offlineExportRunJobFn = runOfflineExportJob
	offlineExportJobCleanupFn = cleanupExpiredOfflineExportJobs
	offlineExportCurrentUserFn = currentUserFromRequest
	offlineExportCurrentRelationFn = GetCurrentRelation
	offlineExportMetricJobsCreated = 0
	offlineExportMetricJobsSucceeded = 0
	offlineExportMetricJobsFailed = 0
	offlineExportMetricJobsPartial = 0
	offlineExportJobsMu.Lock()
	offlineExportJobs = map[string]offlineExportJobContract{}
	offlineExportArtifacts = map[string]map[string]offlineRenderedArtifact{}
	offlineExportJobsMu.Unlock()
}

func TestOfflineExportAllowedFormatsListUsesRolloutOrder(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	if err := os.Setenv("OFFLINE_EXPORT_FORMATS", "notion,image,text"); err != nil {
		t.Fatalf("setenv: %v", err)
	}

	formats := OfflineExportAllowedFormatsList()
	expected := []string{"text", "image", "notion"}
	if len(formats) != len(expected) {
		t.Fatalf("expected %d formats, got %d (%v)", len(expected), len(formats), formats)
	}
	for index := range expected {
		if formats[index] != expected[index] {
			t.Fatalf("expected rollout order %v, got %v", expected, formats)
		}
	}
}

func TestCreateOfflineExportHandlerHonorsFeatureFlagAndAllowedFormats(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	offlineExportCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 33}, Username: "walter"}
	}
	offlineExportCurrentRelationFn = func(user dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 22}}, nil
	}
	offlineExportLoadRecordsFn = func(relationID uint, filters offlineExportFilters) ([]dbmodel.Marker, []dbmodel.Schedule, error) {
		return []dbmodel.Marker{}, []dbmodel.Schedule{}, nil
	}
	offlineExportBuildSnapshotFn = func(input BuildExportSnapshotInput) (*ExportSnapshot, error) {
		return &ExportSnapshot{
			SchemaVersion: ExportSnapshotSchemaVersion,
			Timezone:      input.Timezone,
			GeneratedAt:   time.Now().UTC(),
			Source:        ExportSnapshotSource{RelationID: input.RelationID, UserID: input.UserID},
		}, nil
	}
	offlineExportRunJobFn = func(jobID string) {}

	if err := os.Setenv("OFFLINE_EXPORT_ENABLE", "false"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	disabledRecorder := httptest.NewRecorder()
	disabledRequest := httptest.NewRequest(http.MethodPost, "/exports", bytes.NewBufferString(`{"formats":["text"],"timezone":"UTC"}`))
	CreateOfflineExportHandler(disabledRecorder, disabledRequest)
	if disabledRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when feature disabled, got %d", disabledRecorder.Code)
	}

	if err := os.Setenv("OFFLINE_EXPORT_ENABLE", "true"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	if err := os.Setenv("OFFLINE_EXPORT_FORMATS", "text"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	unsupportedRecorder := httptest.NewRecorder()
	unsupportedRequest := httptest.NewRequest(http.MethodPost, "/exports", bytes.NewBufferString(`{"formats":["image"],"timezone":"UTC"}`))
	CreateOfflineExportHandler(unsupportedRecorder, unsupportedRequest)
	if unsupportedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for disabled format, got %d body=%s", unsupportedRecorder.Code, unsupportedRecorder.Body.String())
	}
}

func TestCreateOfflineExportHandlerAcceptedContract(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	if err := os.Setenv("OFFLINE_EXPORT_FORMATS", "text,image,notion"); err != nil {
		t.Fatalf("setenv: %v", err)
	}

	offlineExportCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 33}, Username: "walter"}
	}
	offlineExportCurrentRelationFn = func(user dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 22}}, nil
	}
	offlineExportLoadRecordsFn = func(relationID uint, filters offlineExportFilters) ([]dbmodel.Marker, []dbmodel.Schedule, error) {
		return []dbmodel.Marker{{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1}}, Label: "A"}}, []dbmodel.Schedule{{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 10}}, Label: "S", SelectedDate: time.Now().UTC()}}, nil
	}
	offlineExportBuildSnapshotFn = func(input BuildExportSnapshotInput) (*ExportSnapshot, error) {
		return &ExportSnapshot{
			SchemaVersion: ExportSnapshotSchemaVersion,
			Timezone:      input.Timezone,
			GeneratedAt:   time.Now().UTC(),
			Source:        ExportSnapshotSource{RelationID: input.RelationID, UserID: input.UserID},
		}, nil
	}
	offlineExportRunJobFn = func(jobID string) {}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/exports", bytes.NewBufferString(`{"formats":["text","image"],"timezone":"UTC"}`))
	CreateOfflineExportHandler(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	response := offlineExportCreateResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid response json: %v", err)
	}
	if response.Job.JobID == "" {
		t.Fatalf("expected job id")
	}
	if response.Job.Snapshot == nil || response.Job.Snapshot.Source.RelationID != 22 {
		t.Fatalf("expected relation-bound snapshot, got %+v", response.Job.Snapshot)
	}
	if response.Job.Artifacts[0].DownloadURL == "" || response.Job.Artifacts[0].DownloadURL[:9] != "/exports/" {
		t.Fatalf("expected user endpoint download url, got %s", response.Job.Artifacts[0].DownloadURL)
	}
}

func TestIntegrationCreateOfflineExportHandlerValidation(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 1}}, ActorUser: dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 2}}}, true
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/exports", bytes.NewBufferString(`{"formats":["pdf"]}`))
	IntegrationCreateOfflineExportHandler(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestIntegrationCreateOfflineExportHandlerAcceptedContract(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	if err := os.Setenv("OFFLINE_EXPORT_FORMATS", "text,image,notion"); err != nil {
		t.Fatalf("setenv: %v", err)
	}

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 22}}, ActorUser: dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 33}}}, true
	}
	offlineExportLoadRecordsFn = func(relationID uint, filters offlineExportFilters) ([]dbmodel.Marker, []dbmodel.Schedule, error) {
		return []dbmodel.Marker{{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 1}}, Label: "A"}}, []dbmodel.Schedule{{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 10}}, Label: "S", SelectedDate: time.Now().UTC()}}, nil
	}
	offlineExportBuildSnapshotFn = func(input BuildExportSnapshotInput) (*ExportSnapshot, error) {
		return &ExportSnapshot{SchemaVersion: ExportSnapshotSchemaVersion, Timezone: input.Timezone, GeneratedAt: time.Now().UTC()}, nil
	}
	offlineExportRunJobFn = func(jobID string) {}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/integration/exports", bytes.NewBufferString(`{"formats":["text","image"],"timezone":"UTC"}`))
	IntegrationCreateOfflineExportHandler(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	response := offlineExportCreateResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid response json: %v", err)
	}
	if response.Job.JobID == "" {
		t.Fatalf("expected job id")
	}
	if len(response.Job.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(response.Job.Artifacts))
	}
	if response.Job.Status != offlineExportJobStatusQueued {
		t.Fatalf("expected queued status, got %s", response.Job.Status)
	}
}

func TestIntegrationGetOfflineExportStatusAndArtifactHandlers(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	integrationAuthenticateRequestFn = func(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
		return &dbmodel.APIKey{Relation: dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 22}}, ActorUser: dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 33}}}, true
	}

	now := time.Now().UTC()
	offlineExportJobsMu.Lock()
	offlineExportJobs["export_42"] = offlineExportJobContract{
		JobID:     "export_42",
		Status:    offlineExportJobStatusSucceeded,
		CreatedAt: now,
		UpdatedAt: now,
		Artifacts: []offlineExportArtifactContract{{Format: "text", Status: "succeeded", DownloadURL: "/integration/exports/export_42/artifacts/text"}},
	}
	offlineExportArtifacts["export_42"] = map[string]offlineRenderedArtifact{
		"text": {
			FileName:    "export_42.txt",
			ContentType: "text/plain; charset=utf-8",
			Content:     []byte("export body"),
		},
	}
	offlineExportJobsMu.Unlock()

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/integration/exports/export_42", nil)
	statusCtx := chi.NewRouteContext()
	statusCtx.URLParams.Add("job_id", "export_42")
	statusRequest = statusRequest.WithContext(context.WithValue(statusRequest.Context(), chi.RouteCtxKey, statusCtx))
	IntegrationGetOfflineExportStatusHandler(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for status, got %d", statusRecorder.Code)
	}

	artifactRecorder := httptest.NewRecorder()
	artifactRequest := httptest.NewRequest(http.MethodGet, "/integration/exports/export_42/artifacts/text", nil)
	artifactCtx := chi.NewRouteContext()
	artifactCtx.URLParams.Add("job_id", "export_42")
	artifactCtx.URLParams.Add("format", "text")
	artifactRequest = artifactRequest.WithContext(context.WithValue(artifactRequest.Context(), chi.RouteCtxKey, artifactCtx))
	IntegrationGetOfflineExportArtifactHandler(artifactRecorder, artifactRequest)
	if artifactRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for artifact, got %d", artifactRecorder.Code)
	}
	if artifactRecorder.Body.String() != "export body" {
		t.Fatalf("expected artifact payload body, got %q", artifactRecorder.Body.String())
	}
}

func TestGetOfflineExportStatusAndArtifactHandlersRequireMatchingRelation(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	now := time.Now().UTC()
	offlineExportJobsMu.Lock()
	offlineExportJobs["export_55"] = offlineExportJobContract{
		JobID:     "export_55",
		Status:    offlineExportJobStatusSucceeded,
		CreatedAt: now,
		UpdatedAt: now,
		Snapshot: &ExportSnapshot{
			SchemaVersion: ExportSnapshotSchemaVersion,
			GeneratedAt:   now,
			Timezone:      "UTC",
			Source:        ExportSnapshotSource{RelationID: 88, UserID: 33},
		},
		Artifacts: []offlineExportArtifactContract{{Format: "text", Status: "succeeded", DownloadURL: "/exports/export_55/artifacts/text"}},
	}
	offlineExportArtifacts["export_55"] = map[string]offlineRenderedArtifact{
		"text": {
			FileName:    "export_55.txt",
			ContentType: "text/plain; charset=utf-8",
			Content:     []byte("export body"),
		},
	}
	offlineExportJobsMu.Unlock()

	offlineExportCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 33}, Username: "walter"}
	}
	offlineExportCurrentRelationFn = func(user dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 88}}, nil
	}

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/exports/export_55", nil)
	statusCtx := chi.NewRouteContext()
	statusCtx.URLParams.Add("job_id", "export_55")
	statusRequest = statusRequest.WithContext(context.WithValue(statusRequest.Context(), chi.RouteCtxKey, statusCtx))
	GetOfflineExportStatusHandler(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for status, got %d", statusRecorder.Code)
	}

	artifactRecorder := httptest.NewRecorder()
	artifactRequest := httptest.NewRequest(http.MethodGet, "/exports/export_55/artifacts/text", nil)
	artifactCtx := chi.NewRouteContext()
	artifactCtx.URLParams.Add("job_id", "export_55")
	artifactCtx.URLParams.Add("format", "text")
	artifactRequest = artifactRequest.WithContext(context.WithValue(artifactRequest.Context(), chi.RouteCtxKey, artifactCtx))
	GetOfflineExportArtifactHandler(artifactRecorder, artifactRequest)
	if artifactRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for artifact, got %d", artifactRecorder.Code)
	}
	if artifactRecorder.Body.String() != "export body" {
		t.Fatalf("expected artifact payload body, got %q", artifactRecorder.Body.String())
	}

	offlineExportCurrentRelationFn = func(user dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 99}}, nil
	}
	missingRecorder := httptest.NewRecorder()
	missingRequest := httptest.NewRequest(http.MethodGet, "/exports/export_55", nil)
	missingRequest = missingRequest.WithContext(context.WithValue(missingRequest.Context(), chi.RouteCtxKey, statusCtx))
	GetOfflineExportStatusHandler(missingRecorder, missingRequest)
	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for mismatched relation, got %d", missingRecorder.Code)
	}
}

func TestRunOfflineExportJobUpdatesPerFormatStatus(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	now := time.Now().UTC()
	offlineExportJobsMu.Lock()
	offlineExportJobs["export_7"] = offlineExportJobContract{
		JobID:     "export_7",
		Status:    offlineExportJobStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
		Snapshot: &ExportSnapshot{
			SchemaVersion: ExportSnapshotSchemaVersion,
			GeneratedAt:   now,
			Timezone:      "UTC",
			Source:        ExportSnapshotSource{RelationID: 22},
		},
		Artifacts: []offlineExportArtifactContract{
			{Format: offlineExportFormatText, Status: offlineExportJobStatusQueued, DownloadURL: "/integration/exports/export_7/artifacts/text"},
			{Format: offlineExportFormatImage, Status: offlineExportJobStatusQueued, DownloadURL: "/integration/exports/export_7/artifacts/image"},
		},
	}
	offlineExportJobsMu.Unlock()

	runOfflineExportJob("export_7")

	offlineExportJobsMu.RLock()
	job := offlineExportJobs["export_7"]
	offlineExportJobsMu.RUnlock()

	if job.Status != offlineExportJobStatusSucceeded {
		t.Fatalf("expected succeeded job, got %s", job.Status)
	}
	if len(job.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(job.Artifacts))
	}
	for _, artifact := range job.Artifacts {
		if artifact.Status != offlineExportJobStatusSucceeded {
			t.Fatalf("expected succeeded artifact status for %s, got %s", artifact.Format, artifact.Status)
		}
	}
	if offlineExportMetricJobsSucceeded != 1 {
		t.Fatalf("expected succeeded metric 1, got %d", offlineExportMetricJobsSucceeded)
	}
}

func TestCleanupExpiredOfflineExportJobsRetainsActiveAndDropsExpiredTerminal(t *testing.T) {
	resetOfflineExportHooks()
	defer resetOfflineExportHooks()

	now := time.Now().UTC()
	offlineExportJobsMu.Lock()
	offlineExportJobs["expired_success"] = offlineExportJobContract{
		JobID:     "expired_success",
		Status:    offlineExportJobStatusSucceeded,
		CreatedAt: now.Add(-48 * time.Hour),
		UpdatedAt: now.Add(-48 * time.Hour),
	}
	offlineExportJobs["expired_processing"] = offlineExportJobContract{
		JobID:     "expired_processing",
		Status:    offlineExportJobStatusProcessing,
		CreatedAt: now.Add(-48 * time.Hour),
		UpdatedAt: now.Add(-48 * time.Hour),
	}
	offlineExportJobs["recent_failed"] = offlineExportJobContract{
		JobID:     "recent_failed",
		Status:    offlineExportJobStatusFailed,
		CreatedAt: now.Add(-1 * time.Hour),
		UpdatedAt: now.Add(-1 * time.Hour),
	}
	cleanupExpiredOfflineExportJobs(now)
	_, hasExpiredSuccess := offlineExportJobs["expired_success"]
	_, hasExpiredProcessing := offlineExportJobs["expired_processing"]
	_, hasRecentFailed := offlineExportJobs["recent_failed"]
	offlineExportJobsMu.Unlock()

	if hasExpiredSuccess {
		t.Fatalf("expected expired terminal job to be cleaned up")
	}
	if !hasExpiredProcessing {
		t.Fatalf("expected expired processing job to remain")
	}
	if !hasRecentFailed {
		t.Fatalf("expected recent terminal job to remain")
	}
}
