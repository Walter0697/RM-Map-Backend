package dbmodel

import "gorm.io/gorm"

type ReleaseNote struct {
	BaseModel
	Version string `json:"version"`
	Notes   string `json:"notes"`
}

func (note *ReleaseNote) Create(db *gorm.DB) error {
	if err := db.Create(note).Error; err != nil {
		return err
	}

	return nil
}

func (note *ReleaseNote) GetReleaseNoteByVersion(db *gorm.DB) error {
	if err := db.Where("version = ?", note.Version).First(note).Error; err != nil {
		return err
	}

	return nil
}

func (note *ReleaseNote) GetLatestRecord(db *gorm.DB) error {
	if err := db.Last(note).Error; err != nil {
		return err
	}

	return nil
}

func (note *ReleaseNote) CheckReleaseRecordExist(db *gorm.DB) bool {
	var count int64
	if err := db.Model(note).Where("version = ?", note.Version).Count(&count).Error; err != nil {
		return false
	}

	return count != 0
}
