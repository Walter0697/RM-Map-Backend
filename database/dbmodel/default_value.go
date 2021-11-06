package dbmodel

import "mapmarker/backend/database"

type DefaultValue struct {
	ObjectBase
	Label   string `json:"label"`
	PinType *Pin   `gorm:"foreignKey:pin_id;reference:id"`
	PinId   *uint
}

func (value *DefaultValue) GetOrCreate() error {
	if err := database.Connection.Where("label = ?", value.Label).FirstOrCreate(value).Error; err != nil {
		return err
	}

	return nil
}

func (value *DefaultValue) GetOrCreatePin() error {
	if err := database.Connection.Preload("PinType").Where("label = ?", value.Label).FirstOrCreate(value).Error; err != nil {
		return err
	}

	return nil
}

func (value *DefaultValue) Update() error {
	if err := database.Connection.Save(value).Error; err != nil {
		return err
	}

	return nil
}
