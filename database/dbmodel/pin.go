package dbmodel

import "gorm.io/gorm"

type Pin struct {
	ObjectBase
	Label        string `json:"label"`
	ImagePath    string `json:"imagePath"`
	DisplayPath  string `json:"displayPath"`
	TopLeftX     int    `json:"topLeftx"`
	TopLeftY     int    `json:"topLefty"`
	BottomRightX int    `json:"bottomRightx"`
	BottomRightY int    `json:"bottomRighty"`
}

func (pin *Pin) Create(db *gorm.DB) error {
	if err := db.Create(pin).Error; err != nil {
		return err
	}

	return nil
}

func (pin *Pin) Update(db *gorm.DB) error {
	if err := db.Save(pin).Error; err != nil {
		return err
	}

	return nil
}

func (pin *Pin) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", pin.ID).First(pin).Error; err != nil {
		return err
	}

	return nil
}

func (pin *Pin) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", pin.ID).Delete(pin).Error; err != nil {
		return err
	}

	return nil
}
