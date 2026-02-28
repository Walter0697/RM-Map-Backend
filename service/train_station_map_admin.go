package service

import (
	"fmt"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"

	"gorm.io/gorm"
)

type TrainStationMapSummary struct {
	MapName      string `json:"map_name"`
	ImagePath    string `json:"image_path"`
	StationCount int64  `json:"station_count"`
	HasImage     bool   `json:"has_image"`
}

func EnsureTrainStationMapExists(mapName string) error {
	name := strings.TrimSpace(mapName)
	if name == "" {
		return fmt.Errorf("map_name is required")
	}

	var item dbmodel.TrainStationMap
	item.MapName = name
	if err := item.GetByMapName(database.Connection); err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		item.ImagePath = ""
		return item.Create(database.Connection)
	}
	return nil
}

func ListTrainStationMaps() ([]TrainStationMapSummary, error) {
	if err := EnsureTrainStationMapExists(constant.HKMTR); err != nil {
		return nil, err
	}

	discovered := make(map[string]bool)
	discovered[constant.HKMTR] = true

	stationMapNames := make([]string, 0)
	if err := database.Connection.Model(&dbmodel.TrainStation{}).Distinct().Pluck("map_name", &stationMapNames).Error; err != nil {
		return nil, err
	}
	for _, name := range stationMapNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || discovered[trimmed] {
			continue
		}
		discovered[trimmed] = true
		if err := EnsureTrainStationMapExists(trimmed); err != nil {
			return nil, err
		}
	}

	recordMapNames := make([]string, 0)
	if err := database.Connection.Model(&dbmodel.DataRecord{}).
		Where("related_table = ?", constant.TrainStation).
		Distinct().
		Pluck("related_name", &recordMapNames).Error; err != nil {
		return nil, err
	}
	for _, name := range recordMapNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || discovered[trimmed] {
			continue
		}
		discovered[trimmed] = true
		if err := EnsureTrainStationMapExists(trimmed); err != nil {
			return nil, err
		}
	}

	items := make([]dbmodel.TrainStationMap, 0)
	if err := database.Connection.Order("map_name asc").Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]TrainStationMapSummary, 0, len(items))
	for _, item := range items {
		var stationCount int64
		if err := database.Connection.Model(&dbmodel.TrainStation{}).
			Where("map_name = ?", item.MapName).
			Count(&stationCount).Error; err != nil {
			return nil, err
		}

		result = append(result, TrainStationMapSummary{
			MapName:      item.MapName,
			ImagePath:    item.ImagePath,
			StationCount: stationCount,
			HasImage:     strings.TrimSpace(item.ImagePath) != "",
		})
	}

	return result, nil
}

func CreateTrainStationMap(mapName string, actor *dbmodel.User) (*dbmodel.TrainStationMap, error) {
	name := strings.TrimSpace(mapName)
	if name == "" {
		return nil, fmt.Errorf("map_name is required")
	}

	item := dbmodel.TrainStationMap{
		ObjectBase: dbmodel.ObjectBase{
			CreatedBy: actor,
			UpdatedBy: actor,
		},
		MapName:   name,
		ImagePath: "",
	}

	if err := item.Create(database.Connection); err != nil {
		return nil, err
	}

	return &item, nil
}

func RemoveTrainStationMap(mapName string) error {
	name := strings.TrimSpace(mapName)
	if name == "" {
		return fmt.Errorf("map_name is required")
	}
	if name == constant.HKMTR {
		return fmt.Errorf("cannot remove built-in map %s", constant.HKMTR)
	}

	var stationCount int64
	if err := database.Connection.Model(&dbmodel.TrainStation{}).
		Where("map_name = ?", name).
		Count(&stationCount).Error; err != nil {
		return err
	}
	if stationCount > 0 {
		return fmt.Errorf("cannot remove map with existing stations")
	}

	var item dbmodel.TrainStationMap
	item.MapName = name
	return item.RemoveByMapName(database.Connection)
}
