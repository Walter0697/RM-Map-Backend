package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type ScheduleReminderDispatchLog struct {
	BaseModel
	UserID         uint      `gorm:"index"`
	RelationID     uint      `gorm:"index"`
	ScheduleID     uint      `gorm:"index"`
	LocalDate      string    `gorm:"size:10;index"`
	ReminderTime   string    `gorm:"size:5"`
	MarkerTimezone string    `gorm:"size:64"`
	DispatchedAt   time.Time `gorm:"index"`
}

func (log *ScheduleReminderDispatchLog) Create(db *gorm.DB) error {
	return db.Create(log).Error
}
