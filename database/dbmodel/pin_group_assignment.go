package dbmodel

import "time"

type PinGroupAssignment struct {
	PinID      uint      `json:"pin_id" gorm:"primaryKey;index:idx_pin_group_assignments_pin_id"`
	PinGroupID uint      `json:"pin_group_id" gorm:"primaryKey;index:idx_pin_group_assignments_group_id"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedUID *uint     `json:"created_uid"`
}
