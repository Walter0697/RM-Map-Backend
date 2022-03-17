package dbmodel

import (
	"mapmarker/backend/utils"

	"gorm.io/gorm"
)

type TrainStation struct {
	ObjectBase
	Label            string  `json:"label"`
	StationLocalName string  `json:"stationLocalName"`
	Identifier       string  `json:"identifier"`
	PhotoX           float64 `json:"photoX"`
	PhotoY           float64 `json:"photoY"`
	MapX             float64 `json:"mapX"`
	MapY             float64 `json:"mapY"`
	LineInfo         string  `json:"lineInfo"`
	MapName          string  `json:"mapName"`
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

func (station *TrainStation) UpdateByMapAndIdentifier(db *gorm.DB) error {
	var temp TrainStation
	temp.Identifier = station.Identifier
	temp.MapName = station.MapName
	if err := db.Where("identifier = ? AND map_name = ?", station.Identifier, station.MapName).First(&temp).Error; err != nil {
		if utils.RecordNotFound(err) {
			err := db.Create(station).Error
			if err != nil {
				return err
			} else {
				return nil
			}
		} else {
			return err
		}
	}

	station.ID = temp.ID
	if err := db.Save(station).Error; err != nil {
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
