package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

const (
	CalendarSyncStatusPending      = "pending"
	CalendarSyncStatusSynced       = "synced"
	CalendarSyncStatusFailed       = "failed"
	CalendarSyncStatusDisconnected = "disconnected"
)

type ScheduleCalendarSyncLink struct {
	ObjectBase
	Schedule           Schedule                   `json:"schedule" gorm:"foreignKey:schedule_id;references:id"`
	ScheduleID         uint                       `json:"schedule_id" gorm:"not null;index;uniqueIndex:idx_schedule_calendar_sync_links_schedule_provider"`
	ProviderKey        string                     `json:"provider_key" gorm:"size:64;not null;index;uniqueIndex:idx_schedule_calendar_sync_links_schedule_provider"`
	Connection         CalendarProviderConnection `json:"connection" gorm:"foreignKey:connection_id;references:id"`
	ConnectionID       uint                       `json:"connection_id" gorm:"not null;index"`
	ExternalEventID    string                     `json:"external_event_id" gorm:"size:190;index"`
	ExternalCalendarID string                     `json:"external_calendar_id" gorm:"size:190"`
	SyncStatus         string                     `json:"sync_status" gorm:"size:32;not null;default:'pending';index"`
	LastSyncedAt       *time.Time                 `json:"last_synced_at"`
	LastErrorCode      string                     `json:"last_error_code" gorm:"size:96"`
	LastErrorMessage   string                     `json:"last_error_message" gorm:"type:text"`
	LastOperationKey   string                     `json:"last_operation_key" gorm:"size:128;index"`
	RetryCount         int                        `json:"retry_count" gorm:"not null;default:0"`
	NextRetryAt        *time.Time                 `json:"next_retry_at"`
	DisconnectedAt     *time.Time                 `json:"disconnected_at"`
}

func (link *ScheduleCalendarSyncLink) Create(db *gorm.DB) error {
	if err := db.Create(link).Error; err != nil {
		return err
	}
	return nil
}

func (link *ScheduleCalendarSyncLink) Update(db *gorm.DB) error {
	if err := db.Save(link).Error; err != nil {
		return err
	}
	return nil
}
