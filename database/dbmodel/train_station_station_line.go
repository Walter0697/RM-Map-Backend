package dbmodel

import "gorm.io/gorm"

type TrainStationStationLine struct {
	ObjectBase
	StationID uint `json:"stationId" gorm:"not null;index;uniqueIndex:idx_train_station_station_lines_unique"`
	LineID    uint `json:"lineId" gorm:"not null;index;uniqueIndex:idx_train_station_station_lines_unique"`
	Position  int  `json:"position" gorm:"not null;default:0"`
}

func (item *TrainStationStationLine) Create(db *gorm.DB) error {
	return db.Create(item).Error
}

func (item *TrainStationStationLine) Update(db *gorm.DB) error {
	return db.Save(item).Error
}
