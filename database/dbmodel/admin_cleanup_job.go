package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type AdminCleanupJob struct {
	ObjectBase
	EntityType   string     `json:"entity_type"`
	TargetID     uint       `json:"target_id"`
	ConfirmLabel string     `json:"confirm_label"`
	Reason       string     `json:"reason"`
	ExecuteAt    time.Time  `json:"execute_at"`
	Status       string     `json:"status"`
	Outcome      string     `json:"outcome"`
	ExecutedAt   *time.Time `json:"executed_at"`
}

func (job *AdminCleanupJob) Create(db *gorm.DB) error {
	if err := db.Create(job).Error; err != nil {
		return err
	}

	return nil
}

func (job *AdminCleanupJob) Update(db *gorm.DB) error {
	if err := db.Save(job).Error; err != nil {
		return err
	}

	return nil
}
