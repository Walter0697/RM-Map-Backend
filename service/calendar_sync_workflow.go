package service

import (
	"fmt"
	"log"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"

	"gorm.io/gorm"
)

func enqueueCalendarSyncMutation(tx *gorm.DB, scheduleID uint, action string) error {
	runtime := getCalendarSyncRuntime()
	schedule := dbmodel.Schedule{}
	schedule.ID = scheduleID
	if err := schedule.GetById(tx); err != nil {
		return err
	}
	version := schedule.UpdatedAt.UTC().Format(time.RFC3339Nano)
	if strings.TrimSpace(version) == "" {
		version = schedule.SelectedDate.UTC().Format(time.RFC3339Nano)
	}
	links := make([]dbmodel.ScheduleCalendarSyncLink, 0)
	if err := tx.Where("schedule_id = ? AND sync_status <> ?", scheduleID, dbmodel.CalendarSyncStatusDisconnected).Find(&links).Error; err != nil {
		return err
	}
	for _, link := range links {
		jobAction := action
		if action == dbmodel.CalendarSyncJobActionUpdate && link.ExternalEventID == "" {
			jobAction = dbmodel.CalendarSyncJobActionCreate
		}
		if action == dbmodel.CalendarSyncJobActionDelete && link.ExternalEventID == "" {
			continue
		}
		if action == dbmodel.CalendarSyncJobActionUpdate {
			if err := runtime.linkRepo.MarkPending(link.ID); err != nil {
				log.Printf("[calendar-sync] mark_pending_failed schedule_id=%d link_id=%d error=%v", scheduleID, link.ID, err)
			}
		}
		if _, err := runtime.jobQueue.Enqueue(CalendarSyncJobEnqueueInput{
			ScheduleID:     scheduleID,
			LinkID:         &link.ID,
			ProviderKey:    link.ProviderKey,
			Action:         jobAction,
			IdempotencyKey: buildCalendarJobKey(jobAction, scheduleID, link.ID, version),
			PayloadJSON:    "{}",
			MaxAttempts:    5,
		}); err != nil {
			return err
		}
	}
	return nil
}

func buildCalendarJobKey(action string, scheduleID uint, linkID uint, version string) string {
	return fmt.Sprintf("%s:%d:%d:%s", strings.TrimSpace(action), scheduleID, linkID, strings.TrimSpace(version))
}
