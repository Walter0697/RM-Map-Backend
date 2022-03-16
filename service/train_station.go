package service

import (
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
)

func GetAllTrainStationByMapName(name string) ([]dbmodel.TrainStation, error) {
	var stations []dbmodel.TrainStation

	query := database.Connection.Where("map_name = ?", name)
	if err := query.Find(&stations).Error; err != nil {
		return stations, err
	}

	return stations, nil
}
