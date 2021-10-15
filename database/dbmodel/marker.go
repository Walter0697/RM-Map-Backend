package dbmodel

import (
	"mapmarker/backend/database"
	"time"
)

type Marker struct {
	ObjectBase
	Label       string     `json:"label"`
	Latitude    string     `json:"latitude"`
	Longitude   string     `json:"longitude"`
	Address     string     `json:"address"`
	ImageLink   string     `json:"imageLink"`
	Link        string     `json:"link"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	FromTime    *time.Time `json:"fromTime"`
	ToTime      *time.Time `json:"toTime"`
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

func (marker *Marker) GetMarkerById() error {
	if err := database.Connection.Where("id = ?", marker.ID).Find(marker).Error; err != nil {
		return err
	}

	return nil
}
