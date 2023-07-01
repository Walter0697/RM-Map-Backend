package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type CountryLocation struct {
	ObjectBase
	Label          string       `json:"label"`
	VisitTime      *time.Time   `json:"visitTime"`
	RelatedPoint   CountryPoint `gorm:"foreignKey:country_point_id;reference:id"`
	CountryPointId uint
	ImageLink      string  `json:"imageLink"`
	RelatedMarker  *Marker `gorm:"foreignKey:marker_id;reference:id"`
	MarkerId       *uint
	Relation       UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId     uint
}

func (location *CountryLocation) Create(db *gorm.DB) error {
	if err := db.Create(location).Error; err != nil {
		return err
	}

	return nil
}

func (location *CountryLocation) Update(db *gorm.DB) error {
	if err := db.Save(location).Error; err != nil {
		return err
	}

	return nil
}

func (location *CountryLocation) GetById(db *gorm.DB) error {
	if err := db.Preload("RelatedPoint").Preload("RelatedMarker").Where("id = ?", location.ID).First(location).Error; err != nil {
		return err
	}

	return nil
}
