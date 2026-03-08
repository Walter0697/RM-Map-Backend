package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

const (
	CalendarSyncJobActionCreate = "create"
	CalendarSyncJobActionUpdate = "update"
	CalendarSyncJobActionDelete = "delete"

	CalendarSyncJobStatusPending    = "pending"
	CalendarSyncJobStatusProcessing = "processing"
	CalendarSyncJobStatusSucceeded  = "succeeded"
	CalendarSyncJobStatusFailed     = "failed"
	CalendarSyncJobStatusDead       = "dead"
)

type CalendarSyncJob struct {
	ObjectBase
	ScheduleID      uint       `json:"schedule_id" gorm:"not null;index"`
	LinkID          *uint      `json:"link_id" gorm:"index"`
	ProviderKey     string     `json:"provider_key" gorm:"size:64;not null;index"`
	Action          string     `json:"action" gorm:"size:32;not null;index"`
	IdempotencyKey  string     `json:"idempotency_key" gorm:"size:128;not null;uniqueIndex"`
	Status          string     `json:"status" gorm:"size:32;not null;default:'pending';index"`
	PayloadJSON     string     `json:"payload_json" gorm:"type:text"`
	Attempts        int        `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts     int        `json:"max_attempts" gorm:"not null;default:5"`
	LastErrorCode   string     `json:"last_error_code" gorm:"size:96"`
	LastErrorDetail string     `json:"last_error_detail" gorm:"type:text"`
	NextAttemptAt   *time.Time `json:"next_attempt_at" gorm:"index"`
	StartedAt       *time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
}

func (job *CalendarSyncJob) Create(db *gorm.DB) error {
	if err := db.Create(job).Error; err != nil {
		return err
	}
	return nil
}

func (job *CalendarSyncJob) Update(db *gorm.DB) error {
	if err := db.Save(job).Error; err != nil {
		return err
	}
	return nil
}
