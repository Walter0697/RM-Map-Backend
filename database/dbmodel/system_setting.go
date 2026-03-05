package dbmodel

import "gorm.io/gorm"

type SystemSetting struct {
	ObjectBase
	Key   string `json:"key" gorm:"uniqueIndex;size:191;not null"`
	Value string `json:"value" gorm:"type:text;not null;default:''"`
}

func (setting *SystemSetting) GetByKey(db *gorm.DB) error {
	return db.Where("key = ?", setting.Key).First(setting).Error
}

func (setting *SystemSetting) UpsertByKey(db *gorm.DB) error {
	return db.Where("key = ?", setting.Key).Assign(map[string]interface{}{
		"value": setting.Value,
	}).FirstOrCreate(setting).Error
}
