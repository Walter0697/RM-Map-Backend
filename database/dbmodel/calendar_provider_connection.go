package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

const (
	CalendarConnectionStatusActive                = "active"
	CalendarConnectionStatusDisconnected          = "disconnected"
	CalendarConnectionStatusReauthorizationNeeded = "reauthorization_required"
)

type CalendarProviderConnection struct {
	ObjectBase
	User                        User       `json:"user" gorm:"foreignKey:user_id;references:id"`
	UserID                      uint       `json:"user_id" gorm:"not null;index;uniqueIndex:idx_calendar_provider_connections_user_provider"`
	ProviderKey                 string     `json:"provider_key" gorm:"size:64;not null;index;uniqueIndex:idx_calendar_provider_connections_user_provider"`
	Status                      string     `json:"status" gorm:"size:48;not null;default:'active';index"`
	GrantedScopes               string     `json:"granted_scopes" gorm:"type:text"`
	AccessTokenEncrypted        string     `json:"access_token_encrypted" gorm:"type:text;not null"`
	RefreshTokenEncrypted       string     `json:"refresh_token_encrypted" gorm:"type:text"`
	TokenExpiresAt              *time.Time `json:"token_expires_at"`
	LastRefreshAt               *time.Time `json:"last_refresh_at"`
	ReauthorizationRequiredAt   *time.Time `json:"reauthorization_required_at"`
	LastConnectionErrorCode     string     `json:"last_connection_error_code" gorm:"size:96"`
	LastConnectionErrorMessage  string     `json:"last_connection_error_message" gorm:"type:text"`
	LastSuccessfulSyncOperation *time.Time `json:"last_successful_sync_operation"`
}

func (connection *CalendarProviderConnection) Create(db *gorm.DB) error {
	if err := db.Create(connection).Error; err != nil {
		return err
	}
	return nil
}

func (connection *CalendarProviderConnection) Update(db *gorm.DB) error {
	if err := db.Save(connection).Error; err != nil {
		return err
	}
	return nil
}
