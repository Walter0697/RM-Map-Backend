package dbmodel

import (
	"mapmarker/backend/utils"

	"gorm.io/gorm"
)

type DataRecord struct {
	BaseModel
	RelatedTable string  `json:"relatedTable"`
	RelatedName  string  `json:"relatedName"`
	Version      float64 `json:"version"`
}

func (record *DataRecord) Create(db *gorm.DB) error {
	if err := db.Create(record).Error; err != nil {
		return err
	}

	return nil
}

func (record *DataRecord) Update(db *gorm.DB) error {
	if err := db.Save(record).Error; err != nil {
		return err
	}

	return nil
}

func (record *DataRecord) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", record.ID).First(record).Error; err != nil {
		return err
	}

	return nil
}

func (record *DataRecord) CheckRecordExist(db *gorm.DB) (bool, error) {
	if err := db.Where("related_table = ? AND related_name = ?", record.RelatedTable, record.RelatedName).First(record).Error; err != nil {
		if utils.RecordNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (record *DataRecord) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", record.ID).Delete(record).Error; err != nil {
		return err
	}

	return nil
}
