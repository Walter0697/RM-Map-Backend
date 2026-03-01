package dbmodel

import "gorm.io/gorm"

type TrainStationLine struct {
	ObjectBase
	MapName   string `json:"mapName" gorm:"not null;index;uniqueIndex:idx_train_station_lines_unique"`
	Name      string `json:"name" gorm:"not null;uniqueIndex:idx_train_station_lines_unique"`
	LocalName string `json:"localName" gorm:"not null;uniqueIndex:idx_train_station_lines_unique"`
	Colour    string `json:"colour" gorm:"not null;uniqueIndex:idx_train_station_lines_unique"`
}

func (item *TrainStationLine) Create(db *gorm.DB) error {
	return db.Create(item).Error
}

func (item *TrainStationLine) Update(db *gorm.DB) error {
	return db.Save(item).Error
}
