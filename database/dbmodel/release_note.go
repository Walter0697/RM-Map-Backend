package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type ReleaseNote struct {
	BaseModel
	Version          string     `json:"version" gorm:"index;not null"`
	Title            string     `json:"title"`
	Content          string     `json:"content" gorm:"type:text"`
	ContentFormat    string     `json:"content_format" gorm:"index"`
	SanitizedContent string     `json:"sanitized_content" gorm:"type:text"`
	PublishState     string     `json:"publish_state" gorm:"index;default:draft"`
	PublishedAt      *time.Time `json:"published_at"`
	ImageRefs        string     `json:"image_refs" gorm:"type:text"`
	UpdatedAt        time.Time  `json:"updated_at"`

	// Legacy fields kept for compatibility with existing GraphQL clients.
	Notes string  `json:"notes" gorm:"type:text"`
	Icon  *string `json:"icon"`
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
	if err := db.Where("publish_state = ?", "published").Order("published_at desc, created_at desc").First(note).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			if fallbackErr := db.Order("created_at desc").First(note).Error; fallbackErr != nil {
				return fallbackErr
			}
			return nil
		}
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
