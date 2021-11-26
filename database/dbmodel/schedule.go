package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type Schedule struct {
	ObjectBase
	Label          string    `json:"label"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	SelectedDate   time.Time `json:"selectedDate"`
	SelectedMarker *Marker   `gorm:"foreignKey:marker_id;reference:id"`
	MarkerId       *uint
	Relation       UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId     uint
}

func (schedule *Schedule) Create(db *gorm.DB) error {
	if err := db.Create(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) Update(db *gorm.DB) error {
	if err := db.Save(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) GetById(db *gorm.DB) error {
	if err := db.Preload("SelectedMarker").Where("id = ?", schedule.ID).First(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", schedule.ID).Delete(schedule).Error; err != nil {
		return err
	}

	return nil
}
