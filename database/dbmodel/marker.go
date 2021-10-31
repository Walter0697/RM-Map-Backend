package dbmodel

import (
	"mapmarker/backend/database"
	"time"
)

type Marker struct {
	ObjectBase
	Label        string       `json:"label"` // required fields
	Latitude     float64      `json:"latitude"`
	Longitude    float64      `json:"longitude"`
	Type         string       `json:"type"`
	Address      string       `json:"address"`
	ImageLink    string       `json:"imageLink"` // preview or link
	Link         string       `json:"link"`
	Description  string       `json:"description"` // optional fields
	EstimateTime string       `json:"estimate"`
	Price        string       `json:"price"`
	Status       string       `json:"status"`   // visited / cancelled / ???
	FromTime     *time.Time   `json:"fromTime"` // time related
	ToTime       *time.Time   `json:"toTime"`
	IsFavourite  bool         `json:"isFavourite"` // pre defined value
	Relation     UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId   uint
}

//TODO: add icon table to add icon

func (marker *Marker) Create() error {
	if err := database.Connection.Create(marker).Error; err != nil {
		return err
	}

	return nil
}

func (marker *Marker) Update() error {
	if err := database.Connection.Save(marker).Error; err != nil {
		return err
	}

	return nil
}

func (marker *Marker) GetById() error {
	if err := database.Connection.Where("id = ?", marker.ID).First(marker).Error; err != nil {
		return err
	}

	return nil
}
