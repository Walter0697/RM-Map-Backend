package dbmodel

import (
	"gorm.io/gorm"
)

type DefaultValue struct {
	ObjectBase
	Label   string `json:"label"`
	PinType *Pin   `gorm:"foreignKey:pin_id;reference:id"`
	PinId   *uint
}

func (value *DefaultValue) GetOrCreate(db *gorm.DB) error {
	if err := db.Where("label = ?", value.Label).FirstOrCreate(value).Error; err != nil {
		return err
	}

	return nil
}

func (value *DefaultValue) GetOrCreatePin(db *gorm.DB) error {
	if err := db.Preload("PinType").Where("label = ?", value.Label).FirstOrCreate(value).Error; err != nil {
		return err
	}

	return nil
}

func (value *DefaultValue) Update(db *gorm.DB) error {
	if err := db.Save(value).Error; err != nil {
		return err
	}

	return nil
}
