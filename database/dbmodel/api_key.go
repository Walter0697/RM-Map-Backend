package dbmodel

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	APIKeyStatusActive  = "active"
	APIKeyStatusRevoked = "revoked"
)

type APIKey struct {
	ObjectBase
	Name          string       `json:"name" gorm:"index"`
	Testing       bool         `json:"testing" gorm:"not null;default:false;index"`
	Prefix        string       `json:"prefix"`
	KeyHash       string       `json:"key_hash"`
	Scopes        string       `json:"scopes"`
	Status        string       `json:"status" gorm:"index"`
	Relation      UserRelation `gorm:"foreignKey:relation_id;references:id"`
	RelationID    uint
	ActorUser     User `gorm:"foreignKey:actor_user_id;references:id"`
	ActorUserID   *uint
	ServiceAccount   *ServiceAccount `gorm:"foreignKey:service_account_id;references:id"`
	ServiceAccountID *uint
	LastUsedAt    *time.Time
	ExpiresAt     *time.Time
	RotatedFromID *uint
}

func (apiKey *APIKey) Create(db *gorm.DB) error {
	return db.Create(apiKey).Error
}

func (apiKey *APIKey) Update(db *gorm.DB) error {
	return db.Save(apiKey).Error
}

func (apiKey *APIKey) TouchLastUsedAt(db *gorm.DB, usedAt time.Time) error {
	return db.Model(&APIKey{}).
		Where("id = ?", apiKey.ID).
		Updates(map[string]interface{}{
			"last_used_at": usedAt,
		}).Error
}

func (apiKey *APIKey) GetByID(db *gorm.DB) error {
	return db.Where("id = ?", apiKey.ID).First(apiKey).Error
}

func (apiKey *APIKey) GetByIDWithRelation(db *gorm.DB) error {
	return db.Where("id = ?", apiKey.ID).
		Preload("Relation").
		Preload("ActorUser").
		Preload("ServiceAccount").
		Preload("ServiceAccount.ActingUser").
		First(apiKey).Error
}

func (apiKey *APIKey) IsActive() bool {
	if apiKey.Status != APIKeyStatusActive {
		return false
	}
	if apiKey.ExpiresAt == nil {
		return true
	}
	return apiKey.ExpiresAt.After(time.Now())
}

func (apiKey *APIKey) ScopeList() []string {
	if strings.TrimSpace(apiKey.Scopes) == "" {
		return nil
	}
	return strings.Split(apiKey.Scopes, ",")
}
