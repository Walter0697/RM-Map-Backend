package dbmodel

import "gorm.io/gorm"

type TrainStation struct {
	ObjectBase
	Label            string `json:"label"`
	StationLocalName string `json:"stationLocalName"`
	StationShortName string `json:"stationShortName"`
	Identifier       string `json:"identifier"`
	PhotoX           string `json:"photoX"`
	PhotoY           string `json:"photoY"`
	MapX             string `json:"mapX"`
	MapY             string `json:"mapY"`
	LineName         string `json:"lineName"`
	LineLocalName    string `json:"lineLocalName"`
	LineColour       string `json:"lineColour"`
	MapName          string `json:"mapName"`
}

func (station *TrainStation) Create(db *gorm.DB) error {
	if err := db.Create(station).Error; err != nil {
		return err
	}

	return nil
}

func (station *TrainStation) Update(db *gorm.DB) error {
	if err := db.Save(station).Error; err != nil {
		return err
	}

	return nil
}

func (station *TrainStation) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", station.ID).First(station).Error; err != nil {
		return err
	}

	return nil
}

func (station *TrainStation) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", station.ID).Delete(station).Error; err != nil {
		return err
	}

	return nil
}
