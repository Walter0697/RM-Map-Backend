package dbmodel

import "time"

type SocialMediaShortcutSetupToken struct {
	BaseModel
	TokenHash  string    `json:"-" gorm:"uniqueIndex;not null"`
	UserID     uint      `gorm:"index;not null"`
	RelationID uint      `gorm:"index;not null"`
	ExpiresAt  time.Time `gorm:"index;not null"`
}
