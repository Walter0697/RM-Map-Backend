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

func GetAllStationRecord(relation dbmodel.UserRelation) ([]dbmodel.TrainRecord, error) {
	var records []dbmodel.TrainRecord
	query := database.Connection

	query = query.Preload("SelectedStation")
	query = query.Where("relation_id = ?", relation.ID)

	if err := query.Find(&records).Error; err != nil {
		return records, err
	}

	return records, nil
}

func IsStationActive(list []dbmodel.TrainRecord, identifier, map_name string) bool {
	for _, record := range list {
		if record.SelectedStation.Identifier == identifier && record.SelectedStation.MapName == map_name {
			return record.Active
		}
	}
	return false
}
