package service

import (
	"fmt"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"
)

type TrainStationLineCatalogItem struct {
	ID        uint   `json:"id"`
	MapName   string `json:"map_name"`
	Name      string `json:"name"`
	LocalName string `json:"local_name"`
	Colour    string `json:"colour"`
}

func ListTrainStationLineCatalog(mapName string) ([]TrainStationLineCatalogItem, error) {
	trimmed := strings.TrimSpace(mapName)
	if trimmed == "" {
		return nil, fmt.Errorf("map_name is required")
	}

	rows := make([]dbmodel.TrainStationLine, 0)
	if err := database.Connection.Where("map_name = ?", trimmed).Order("name asc, local_name asc").Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]TrainStationLineCatalogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, TrainStationLineCatalogItem{
			ID:        row.ID,
			MapName:   row.MapName,
			Name:      row.Name,
			LocalName: row.LocalName,
			Colour:    row.Colour,
		})
	}
	return items, nil
}

func SaveTrainStationLineCatalog(mapName string, lines []TrainStationLineCatalogItem) ([]TrainStationLineCatalogItem, error) {
	trimmed := strings.TrimSpace(mapName)
	if trimmed == "" {
		return nil, fmt.Errorf("map_name is required")
	}
	if err := EnsureTrainStationMapExists(trimmed); err != nil {
		return nil, err
	}

	tx := database.Connection.Begin()
	existing := make([]dbmodel.TrainStationLine, 0)
	if err := tx.Where("map_name = ?", trimmed).Find(&existing).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	existingByID := make(map[uint]dbmodel.TrainStationLine, len(existing))
	keepIDs := make(map[uint]bool)
	for _, row := range existing {
		existingByID[row.ID] = row
	}

	for _, input := range lines {
		name := strings.TrimSpace(input.Name)
		localName := strings.TrimSpace(input.LocalName)
		colour := strings.TrimSpace(input.Colour)
		if name == "" {
			continue
		}

		if input.ID != 0 {
			row, ok := existingByID[input.ID]
			if ok {
				row.Name = name
				row.LocalName = localName
				row.Colour = colour
				if err := tx.Save(&row).Error; err != nil {
					tx.Rollback()
					return nil, err
				}
				keepIDs[row.ID] = true
				continue
			}
		}

		row := dbmodel.TrainStationLine{
			MapName:   trimmed,
			Name:      name,
			LocalName: localName,
			Colour:    colour,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		keepIDs[row.ID] = true
	}

	for _, row := range existing {
		if keepIDs[row.ID] {
			continue
		}
		if err := tx.Unscoped().Where("line_id = ?", row.ID).Delete(&dbmodel.TrainStationStationLine{}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Unscoped().Delete(&row).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := RebuildStationLineInfoForMapTx(tx, trimmed); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return ListTrainStationLineCatalog(trimmed)
}
