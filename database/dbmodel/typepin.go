package dbmodel

import "mapmarker/backend/database"

type TypePin struct {
	ObjectBase
	RelatedPin  Pin `gorm:"foreignKey:pin_id;reference:id"`
	PinId       uint
	RelatedType MarkerType `gorm:"foreignKey:type_id;reference:id"`
	TypeId      uint
	ImagePath   string
}

func (typePin *TypePin) Create() error {
	if err := database.Connection.Create(typePin).Error; err != nil {
		return err
	}

	return nil
}

func (typePin *TypePin) Update() error {
	if err := database.Connection.Save(typePin).Error; err != nil {
		return err
	}

	return nil
}

func (typePin *TypePin) GetOrCreate() error {
	if err := database.Connection.Where("pin_id = ? AND type_id = ?", typePin.PinId, typePin.TypeId).FirstOrCreate(typePin).Error; err != nil {
		return err
	}

	return nil
}

func (typePin *TypePin) GetFull() error {
	if err := database.Connection.
		Preload("RelatedPin").
		Preload("RelatedType").
		Where("pin_id = ? AND type_id = ?", typePin.PinId, typePin.TypeId).
		First(typePin).Error; err != nil {
		return err
	}
	return nil
}
