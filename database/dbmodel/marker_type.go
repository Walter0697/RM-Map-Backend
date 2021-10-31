package dbmodel

import "mapmarker/backend/database"

type MarkerType struct {
	ObjectBase
	Label    string `json:"label"`
	Value    string `json:"value"`
	Priority int    `json:"priority"`
	IconPath string `json:"iconPath"`
}

func (marker_type *MarkerType) Create() error {
	if err := database.Connection.Create(marker_type).Error; err != nil {
		return err
	}

	return nil
}

func (marker_type *MarkerType) Update() error {
	if err := database.Connection.Save(marker_type).Error; err != nil {
		return err
	}

	return nil
}

func (marker_type *MarkerType) GetById() error {
	if err := database.Connection.Where("id = ?", marker_type.ID).First(marker_type).Error; err != nil {
		return err
	}

	return nil
}

func (marker_type *MarkerType) RemoveById() error {
	if err := database.Connection.Where("id = ?", marker_type.ID).Delete(marker_type).Error; err != nil {
		return err
	}

	return nil
}
