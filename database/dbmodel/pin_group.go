package dbmodel

import "gorm.io/gorm"

type PinGroup struct {
	ObjectBase
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName" gorm:"index:idx_pin_groups_normalized_name,unique"`
	Pins           []Pin  `json:"pins,omitempty" gorm:"many2many:pin_group_assignments;"`
}

func (group *PinGroup) Create(db *gorm.DB) error {
	if err := db.Create(group).Error; err != nil {
		return err
	}
	return nil
}

func (group *PinGroup) Update(db *gorm.DB) error {
	if err := db.Save(group).Error; err != nil {
		return err
	}
	return nil
}

func (group *PinGroup) GetByID(db *gorm.DB) error {
	if err := db.Where("id = ?", group.ID).First(group).Error; err != nil {
		return err
	}
	return nil
}

func (group *PinGroup) RemoveByID(db *gorm.DB) error {
	if err := db.Where("id = ?", group.ID).Delete(group).Error; err != nil {
		return err
	}
	return nil
}
