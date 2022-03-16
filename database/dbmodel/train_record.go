package dbmodel

import "gorm.io/gorm"

type TrainRecord struct {
	ObjectBase
	SelectedStation TrainStation `gorm:"foreignKey:station_id;reference:id"`
	StationId       uint
	Relation        UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId      uint
}

func (record *TrainRecord) Create(db *gorm.DB) error {
	if err := db.Create(record).Error; err != nil {
		return err
	}

	return nil
}

func (record *TrainRecord) Update(db *gorm.DB) error {
	if err := db.Save(record).Error; err != nil {
		return err
	}

	return nil
}

func (record *TrainRecord) GetById(db *gorm.DB) error {
	if err := db.Preload("SelectedStation").Where("id = ?", record.ID).First(record).Error; err != nil {
		return err
	}

	return nil
}

func (record *TrainRecord) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", record.ID).Delete(record).Error; err != nil {
		return err
	}

	return nil
}
