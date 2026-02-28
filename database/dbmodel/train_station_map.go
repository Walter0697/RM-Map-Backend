package dbmodel

import (
	"strings"

	"gorm.io/gorm"
)

type TrainStationMap struct {
	ObjectBase
	MapName   string `json:"mapName" gorm:"uniqueIndex"`
	ImagePath string `json:"imagePath"`
}

func (item *TrainStationMap) GetByMapName(db *gorm.DB) error {
	return db.Where("map_name = ?", strings.TrimSpace(item.MapName)).First(item).Error
}

func (item *TrainStationMap) Create(db *gorm.DB) error {
	return db.Create(item).Error
}

func (item *TrainStationMap) Update(db *gorm.DB) error {
	return db.Save(item).Error
}

func (item *TrainStationMap) RemoveByMapName(db *gorm.DB) error {
	return db.Where("map_name = ?", strings.TrimSpace(item.MapName)).Delete(item).Error
}
