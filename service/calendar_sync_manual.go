package service

import (
	"context"
	"fmt"
	"log"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"
)

type calendarManualSyncResult struct {
	SyncStatus       string `json:"sync_status"`
	Action           string `json:"action"`
	Provider         string `json:"provider"`
	ExternalEventID  string `json:"external_event_id,omitempty"`
	ExternalCalendar string `json:"external_calendar_id,omitempty"`
	LastErrorCode    string `json:"last_error_code,omitempty"`
	LastErrorMessage string `json:"last_error_message,omitempty"`
}

func executeManualCalendarSync(ctx context.Context, userID uint, scheduleID uint, providerKey string, requestedAction string) (*calendarManualSyncResult, error) {
	runtime := getCalendarSyncRuntime()
	adapter, err := runtime.orchestrator.ResolveAdapter(providerKey)
	if err != nil {
		return nil, fmt.Errorf("provider %s is unavailable", providerKey)
	}
	connection, err := runtime.connectionSvc.GetByUserAndProvider(userID, providerKey)
	if err != nil {
		return nil, fmt.Errorf("provider connection not found")
	}
	if strings.TrimSpace(connection.Status) != dbmodel.CalendarConnectionStatusActive {
		return nil, fmt.Errorf("provider connection is not active")
	}
	link, err := runtime.linkRepo.UpsertLink(CalendarSyncLinkUpsertInput{
		ScheduleID:   scheduleID,
		ProviderKey:  providerKey,
		ConnectionID: connection.ID,
	})
	if err != nil {
		return nil, err
	}

	action := strings.TrimSpace(requestedAction)
	if action == "" {
		action = dbmodel.CalendarSyncJobActionCreate
		if strings.TrimSpace(link.ExternalEventID) != "" {
			action = dbmodel.CalendarSyncJobActionUpdate
		}
	}
	if action == dbmodel.CalendarSyncJobActionUpdate && strings.TrimSpace(link.ExternalEventID) == "" {
		action = dbmodel.CalendarSyncJobActionCreate
	}
	if action == dbmodel.CalendarSyncJobActionDelete && strings.TrimSpace(link.ExternalEventID) == "" {
		_ = runtime.linkRepo.MarkDisconnected(link.ID)
		log.Printf("[calendar-sync-manual] schedule_id=%d user_id=%d provider=%s action=%s status=disconnected reason=no_external_event_id", scheduleID, userID, providerKey, action)
		return &calendarManualSyncResult{
			SyncStatus: dbmodel.CalendarSyncStatusDisconnected,
			Action:     action,
			Provider:   providerKey,
		}, nil
	}

	var request CalendarEventUpsertRequest
	if action != dbmodel.CalendarSyncJobActionDelete {
		schedule := dbmodel.Schedule{}
		schedule.ID = scheduleID
		if err := schedule.GetById(database.Connection); err != nil {
			return nil, err
		}
		request = buildCalendarEventRequestFromSchedule(schedule)
	}
	startAtLog := ""
	titleLog := ""
	timezoneLog := ""
	if action != dbmodel.CalendarSyncJobActionDelete {
		startAtLog = request.StartAt.UTC().Format(time.RFC3339)
		titleLog = truncateCalendarLogValue(strings.TrimSpace(request.Title), 120)
		timezoneLog = strings.TrimSpace(request.Timezone)
	}
	log.Printf(
		"[calendar-sync-manual] schedule_id=%d user_id=%d provider=%s action=%s link_id=%d external_event_id=%q start_at_utc=%s timezone=%s title=%q",
		scheduleID,
		userID,
		providerKey,
		action,
		link.ID,
		strings.TrimSpace(link.ExternalEventID),
		startAtLog,
		timezoneLog,
		titleLog,
	)

	var opErr error
	result := &calendarManualSyncResult{
		Action:   action,
		Provider: providerKey,
	}
	switch action {
	case dbmodel.CalendarSyncJobActionCreate:
		createResult, createErr := adapter.CreateEvent(ctx, *connection, request)
		opErr = createErr
		if opErr == nil {
			_ = runtime.linkRepo.MarkSynced(link.ID, createResult.ExternalEventID, createResult.ExternalCalendarID)
			result.SyncStatus = dbmodel.CalendarSyncStatusSynced
			result.ExternalEventID = strings.TrimSpace(createResult.ExternalEventID)
			result.ExternalCalendar = strings.TrimSpace(createResult.ExternalCalendarID)
		}
	case dbmodel.CalendarSyncJobActionUpdate:
		opErr = adapter.UpdateEvent(ctx, *connection, strings.TrimSpace(link.ExternalEventID), request)
		if opErr == nil {
			_ = runtime.linkRepo.MarkSynced(link.ID, strings.TrimSpace(link.ExternalEventID), strings.TrimSpace(link.ExternalCalendarID))
			result.SyncStatus = dbmodel.CalendarSyncStatusSynced
			result.ExternalEventID = strings.TrimSpace(link.ExternalEventID)
			result.ExternalCalendar = strings.TrimSpace(link.ExternalCalendarID)
		}
	case dbmodel.CalendarSyncJobActionDelete:
		opErr = adapter.DeleteEvent(ctx, *connection, strings.TrimSpace(link.ExternalEventID))
		if opErr == nil {
			_ = runtime.linkRepo.MarkDisconnected(link.ID)
			result.SyncStatus = dbmodel.CalendarSyncStatusDisconnected
			result.ExternalEventID = strings.TrimSpace(link.ExternalEventID)
		}
	default:
		return nil, fmt.Errorf("unsupported action %s", action)
	}

	if opErr == nil {
		log.Printf("[calendar-sync-manual] schedule_id=%d user_id=%d provider=%s action=%s status=%s external_event_id=%q", scheduleID, userID, providerKey, action, result.SyncStatus, result.ExternalEventID)
		calendarAudit("sync_manual_completed", map[string]string{
			"action":      action,
			"provider":    providerKey,
			"schedule_id": fmt.Sprintf("%d", scheduleID),
			"status":      result.SyncStatus,
			"event_id":    result.ExternalEventID,
			"user_id":     fmt.Sprintf("%d", userID),
		})
		return result, nil
	}

	providerCode := "sync_failed"
	providerMessage := opErr.Error()
	nextRetry := time.Now().UTC().Add(30 * time.Second)
	if providerErr, ok := opErr.(*CalendarProviderOperationError); ok {
		providerCode = strings.TrimSpace(providerErr.Code)
		providerMessage = strings.TrimSpace(providerErr.Message)
		if providerErr.ReauthRequired {
			_ = runtime.connectionSvc.MarkReauthorizationRequired(connection.ID, providerCode, providerMessage)
		}
	}
	_ = runtime.linkRepo.MarkFailed(link.ID, providerCode, providerMessage, &nextRetry)
	result.SyncStatus = dbmodel.CalendarSyncStatusFailed
	result.LastErrorCode = providerCode
	result.LastErrorMessage = providerMessage
	log.Printf(
		"[calendar-sync-manual] schedule_id=%d user_id=%d provider=%s action=%s status=failed error_code=%s error_message=%q",
		scheduleID,
		userID,
		providerKey,
		action,
		providerCode,
		truncateCalendarLogValue(providerMessage, 300),
	)
	calendarAudit("sync_manual_failed", map[string]string{
		"action":      action,
		"provider":    providerKey,
		"schedule_id": fmt.Sprintf("%d", scheduleID),
		"error_code":  providerCode,
		"user_id":     fmt.Sprintf("%d", userID),
	})
	return result, nil
}
