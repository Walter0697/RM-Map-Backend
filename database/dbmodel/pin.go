package dbmodel

import "mapmarker/backend/database"

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

func (pin *Pin) Create() error {
	if err := database.Connection.Create(pin).Error; err != nil {
		return err
	}

	return nil
}

func (pin *Pin) Update() error {
	if err := database.Connection.Save(pin).Error; err != nil {
		return err
	}

	return nil
}

func (pin *Pin) GetById() error {
	if err := database.Connection.Where("id = ?", pin.ID).First(pin).Error; err != nil {
		return err
	}

	return nil
}

func (pin *Pin) RemoveById() error {
	if err := database.Connection.Where("id = ?", pin.ID).Delete(pin).Error; err != nil {
		return err
	}

	return nil
}
