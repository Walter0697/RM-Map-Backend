package dbmodel

import (
	"mapmarker/backend/database"
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

func (schedule *Schedule) Create() error {
	if err := database.Connection.Create(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) Update() error {
	if err := database.Connection.Save(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) UpdateWithTransaction(tx *gorm.DB) error {
	if err := tx.Save(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) GetById() error {
	if err := database.Connection.Where("id = ?", schedule.ID).First(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) GetByIdWithTransaction(tx *gorm.DB) error {
	if err := tx.Preload("SelectedMarker").Where("id = ?", schedule.ID).First(schedule).Error; err != nil {
		return err
	}

	return nil
}
