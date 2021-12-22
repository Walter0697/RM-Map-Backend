package dbmodel

import (
	"gorm.io/gorm"
)

type MarkerType struct {
	ObjectBase
	Label    string `json:"label"`
	Value    string `json:"value"`
	Priority int    `json:"priority"`
	IconPath string `json:"iconPath"`
	Hidden   bool   `json:"hidden"`
}

func (marker_type *MarkerType) Create(db *gorm.DB) error {
	if err := db.Create(marker_type).Error; err != nil {
		return err
	}

	return nil
}

func (marker_type *MarkerType) Update(db *gorm.DB) error {
	if err := db.Save(marker_type).Error; err != nil {
		return err
	}

	return nil
}

func (marker_type *MarkerType) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", marker_type.ID).First(marker_type).Error; err != nil {
		return err
	}

	return nil
}

func (marker_type *MarkerType) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", marker_type.ID).Delete(marker_type).Error; err != nil {
		return err
	}

	return nil
}
