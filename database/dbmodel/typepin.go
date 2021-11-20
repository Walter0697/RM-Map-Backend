package dbmodel

import "gorm.io/gorm"

type TypePin struct {
	ObjectBase
	RelatedPin  Pin `gorm:"foreignKey:pin_id;reference:id"`
	PinId       uint
	RelatedType MarkerType `gorm:"foreignKey:type_id;reference:id"`
	TypeId      uint
	ImagePath   string
}

func (typePin *TypePin) Create(db *gorm.DB) error {
	if err := db.Create(typePin).Error; err != nil {
		return err
	}

	return nil
}

func (typePin *TypePin) Update(db *gorm.DB) error {
	if err := db.Save(typePin).Error; err != nil {
		return err
	}

	return nil
}

func (typePin *TypePin) GetOrCreate(db *gorm.DB) error {
	if err := db.Where("pin_id = ? AND type_id = ?", typePin.PinId, typePin.TypeId).FirstOrCreate(typePin).Error; err != nil {
		return err
	}

	return nil
}

func (typePin *TypePin) GetFull(db *gorm.DB) error {
	if err := db.
		Preload("RelatedPin").
		Preload("RelatedType").
		Where("pin_id = ? AND type_id = ?", typePin.PinId, typePin.TypeId).
		First(typePin).Error; err != nil {
		return err
	}
	return nil
}
