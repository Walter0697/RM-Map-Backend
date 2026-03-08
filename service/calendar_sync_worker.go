package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"
)

func StartCalendarSyncWorker() {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			if err := RunCalendarSyncWorkerBatch(context.Background(), 20); err != nil {
				log.Printf("[calendar-sync] worker_batch_error=%v", err)
			}
			<-ticker.C
		}
	}()
}

func RunCalendarSyncWorkerBatch(ctx context.Context, limit int) error {
	runtime := getCalendarSyncRuntime()
	jobs, err := runtime.jobQueue.ClaimDueJobs(limit)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if processErr := processCalendarSyncJob(ctx, runtime, job); processErr != nil {
			log.Printf("[calendar-sync] job_id=%d process_error=%v", job.ID, processErr)
		}
	}
	return nil
}

func processCalendarSyncJob(ctx context.Context, runtime *calendarSyncRuntimeState, job dbmodel.CalendarSyncJob) error {
	adapter, err := runtime.orchestrator.ResolveAdapter(job.ProviderKey)
	if err != nil {
		_ = runtime.jobQueue.MarkFailed(job.ID, "provider_not_found", err.Error())
		return err
	}

	link := &dbmodel.ScheduleCalendarSyncLink{}
	if job.LinkID != nil {
		if err := database.Connection.Where("id = ?", *job.LinkID).First(link).Error; err != nil {
			_ = runtime.jobQueue.MarkFailed(job.ID, "link_not_found", err.Error())
			return err
		}
	}
	connection := dbmodel.CalendarProviderConnection{}
	if link.ID != 0 {
		if err := database.Connection.Where("id = ?", link.ConnectionID).First(&connection).Error; err != nil {
			_ = runtime.jobQueue.MarkFailed(job.ID, "connection_not_found", err.Error())
			return err
		}
	}

	request := CalendarEventUpsertRequest{}
	if strings.TrimSpace(job.Action) != dbmodel.CalendarSyncJobActionDelete {
		schedule := dbmodel.Schedule{}
		schedule.ID = job.ScheduleID
		if err := schedule.GetById(database.Connection); err != nil {
			_ = runtime.jobQueue.MarkFailed(job.ID, "schedule_not_found", err.Error())
			return err
		}
		request = CalendarEventUpsertRequest{
			Title:       schedule.Label,
			Description: schedule.Description,
			StartAt:     schedule.SelectedDate,
		}
		if schedule.SelectedMarker != nil {
			request.Location = strings.TrimSpace(schedule.SelectedMarker.Label)
		}
	}

	var opErr error
	switch strings.TrimSpace(job.Action) {
	case dbmodel.CalendarSyncJobActionCreate:
		result, createErr := adapter.CreateEvent(ctx, connection, request)
		opErr = createErr
		if createErr == nil && link.ID != 0 {
			_ = runtime.linkRepo.MarkSynced(link.ID, result.ExternalEventID, result.ExternalCalendarID)
		}
	case dbmodel.CalendarSyncJobActionUpdate:
		opErr = adapter.UpdateEvent(ctx, connection, link.ExternalEventID, request)
		if opErr == nil && link.ID != 0 {
			_ = runtime.linkRepo.MarkSynced(link.ID, link.ExternalEventID, link.ExternalCalendarID)
		}
	case dbmodel.CalendarSyncJobActionDelete:
		opErr = adapter.DeleteEvent(ctx, connection, link.ExternalEventID)
		if opErr == nil && link.ID != 0 {
			_ = runtime.linkRepo.MarkDisconnected(link.ID)
		}
	default:
		opErr = fmt.Errorf("unsupported sync job action %s", job.Action)
	}
	if opErr == nil {
		log.Printf("[calendar-sync] job_id=%d provider=%s action=%s status=success", job.ID, job.ProviderKey, job.Action)
		calendarMetricJobSucceeded()
		calendarAudit("sync_job_succeeded", map[string]string{
			"action":      job.Action,
			"job_id":      fmt.Sprintf("%d", job.ID),
			"provider":    job.ProviderKey,
			"schedule_id": fmt.Sprintf("%d", job.ScheduleID),
		})
		return runtime.jobQueue.MarkSucceeded(job.ID)
	}

	var providerErr *CalendarProviderOperationError
	if errors.As(opErr, &providerErr) {
		if providerErr.ReconciledSuccess {
			if link.ID != 0 {
				_ = runtime.linkRepo.MarkDisconnected(link.ID)
			}
			return runtime.jobQueue.MarkSucceeded(job.ID)
		}
		if providerErr.ReauthRequired {
			calendarMetricReauthRequired()
			calendarAudit("provider_reauthorization_required", map[string]string{
				"action":      job.Action,
				"job_id":      fmt.Sprintf("%d", job.ID),
				"provider":    job.ProviderKey,
				"schedule_id": fmt.Sprintf("%d", job.ScheduleID),
			})
			if connection.ID != 0 {
				_ = runtime.connectionSvc.MarkReauthorizationRequired(connection.ID, providerErr.Code, providerErr.Message)
			}
		}
		if link.ID != 0 {
			nextRetry := time.Now().UTC().Add(calendarSyncRetryBackoff(job.Attempts))
			_ = runtime.linkRepo.MarkFailed(link.ID, providerErr.Code, providerErr.Message, &nextRetry)
		}
		log.Printf("[calendar-sync] job_id=%d provider=%s action=%s status=failed code=%s retryable=%t reauth=%t", job.ID, job.ProviderKey, job.Action, providerErr.Code, providerErr.Retryable, providerErr.ReauthRequired)
		calendarMetricJobFailed()
		calendarAudit("sync_job_failed", map[string]string{
			"action":      job.Action,
			"code":        providerErr.Code,
			"job_id":      fmt.Sprintf("%d", job.ID),
			"provider":    job.ProviderKey,
			"schedule_id": fmt.Sprintf("%d", job.ScheduleID),
		})
		return runtime.jobQueue.MarkFailed(job.ID, providerErr.Code, providerErr.Message)
	}

	if link.ID != 0 {
		nextRetry := time.Now().UTC().Add(calendarSyncRetryBackoff(job.Attempts))
		_ = runtime.linkRepo.MarkFailed(link.ID, "sync_failed", opErr.Error(), &nextRetry)
	}
	log.Printf("[calendar-sync] job_id=%d provider=%s action=%s status=failed code=sync_failed", job.ID, job.ProviderKey, job.Action)
	calendarMetricJobFailed()
	calendarAudit("sync_job_failed", map[string]string{
		"action":      job.Action,
		"code":        "sync_failed",
		"job_id":      fmt.Sprintf("%d", job.ID),
		"provider":    job.ProviderKey,
		"schedule_id": fmt.Sprintf("%d", job.ScheduleID),
	})
	return runtime.jobQueue.MarkFailed(job.ID, "sync_failed", opErr.Error())
}
