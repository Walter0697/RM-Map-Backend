package dbmodel

import "gorm.io/gorm"

const (
	MarkerCreationOutcomeStatusSuccess = "success"
	MarkerCreationOutcomeStatusFailed  = "failed"
)

type MarkerCreationOutcomeLog struct {
	BaseModel
	Link           string  `json:"link" gorm:"not null;index:idx_marker_creation_outcome_logs_link"`
	Status         string  `json:"status" gorm:"not null;index:idx_marker_creation_outcome_logs_status"`
	MarkerID       *uint   `json:"marker_id" gorm:"index"`
	ExternalRunID  *string `json:"external_run_id" gorm:"index:idx_marker_creation_outcome_logs_external_run_id"`
	FailureReason  *string `json:"failure_reason"`
	FailureMessage *string `json:"failure_message"`
}

func (logEntry *MarkerCreationOutcomeLog) Create(db *gorm.DB) error {
	return db.Create(logEntry).Error
}
