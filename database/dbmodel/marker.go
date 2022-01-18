package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type Marker struct {
	ObjectBase
	Label          string      `json:"label"` // required fields
	Latitude       float64     `json:"latitude"`
	Longitude      float64     `json:"longitude"`
	Type           string      `json:"type"`
	Address        string      `json:"address"`
	ImageLink      string      `json:"imageLink"` // preview or link
	Link           string      `json:"link"`
	Description    string      `json:"description"` // optional fields
	EstimateTime   string      `json:"estimate"`
	Price          string      `json:"price"`
	Permanent      bool        `json:"permanent"`
	NeedBooking    bool        `json:"needBooking"`
	Status         string      `json:"status"`    // arrived / scheduled / ???
	VisitTime      *time.Time  `json:"visitTime"` // DEPRECATED FIELD
	FromTime       *time.Time  `json:"fromTime"`  // time related
	ToTime         *time.Time  `json:"toTime"`
	RestaurantInfo *Restaurant `gorm:"foreignKey:restaurant_id;reference:id"`
	RestaurantId   *uint
	IsFavourite    bool         `json:"isFavourite"` // pre defined value
	Relation       UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId     uint
}

func (marker *Marker) Create(db *gorm.DB) error {
	if err := db.Create(marker).Error; err != nil {
		return err
	}

	return nil
}

func (marker *Marker) Update(db *gorm.DB) error {
	if err := db.Save(marker).Error; err != nil {
		return err
	}

	return nil
}

func (marker *Marker) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", marker.ID).First(marker).Error; err != nil {
		return err
	}

	return nil
}

func (marker *Marker) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", marker.ID).Delete(marker).Error; err != nil {
		return err
	}

	return nil
}
