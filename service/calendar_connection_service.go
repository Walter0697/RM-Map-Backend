package service

import (
	"errors"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrCalendarConnectionNotFound = errors.New("calendar connection not found")

type CalendarConnectionUpsertInput struct {
	UserID                uint
	ProviderKey           string
	GrantedScopes         []string
	AccessTokenEncrypted  string
	RefreshTokenEncrypted string
	TokenExpiresAt        *time.Time
}

type CalendarConnectionService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewCalendarConnectionService(db *gorm.DB) *CalendarConnectionService {
	return &CalendarConnectionService{
		db:  db,
		now: time.Now,
	}
}

func (service *CalendarConnectionService) GetByUserAndProvider(userID uint, providerKey string) (*dbmodel.CalendarProviderConnection, error) {
	connection := &dbmodel.CalendarProviderConnection{}
	err := service.db.
		Where("user_id = ? AND provider_key = ?", userID, normalizeCalendarProviderKey(providerKey)).
		First(connection).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCalendarConnectionNotFound
		}
		return nil, err
	}
	return connection, nil
}

func (service *CalendarConnectionService) UpsertAuthorizedConnection(input CalendarConnectionUpsertInput) (*dbmodel.CalendarProviderConnection, error) {
	connection, err := service.GetByUserAndProvider(input.UserID, input.ProviderKey)
	now := service.now().UTC()
	if err != nil && !errors.Is(err, ErrCalendarConnectionNotFound) {
		return nil, err
	}

	scopes := strings.Join(normalizeCalendarScopes(input.GrantedScopes), " ")
	if errors.Is(err, ErrCalendarConnectionNotFound) {
		connection = &dbmodel.CalendarProviderConnection{
			UserID:                input.UserID,
			ProviderKey:           normalizeCalendarProviderKey(input.ProviderKey),
			Status:                dbmodel.CalendarConnectionStatusActive,
			GrantedScopes:         scopes,
			AccessTokenEncrypted:  strings.TrimSpace(input.AccessTokenEncrypted),
			RefreshTokenEncrypted: strings.TrimSpace(input.RefreshTokenEncrypted),
			TokenExpiresAt:        input.TokenExpiresAt,
			LastRefreshAt:         &now,
		}
		if err := connection.Create(service.db); err != nil {
			return nil, err
		}
		return connection, nil
	}

	if !canTransitionCalendarConnectionStatus(strings.TrimSpace(connection.Status), dbmodel.CalendarConnectionStatusActive) {
		return nil, ErrCalendarConnectionNotFound
	}
	connection.Status = dbmodel.CalendarConnectionStatusActive
	connection.GrantedScopes = scopes
	connection.AccessTokenEncrypted = strings.TrimSpace(input.AccessTokenEncrypted)
	connection.RefreshTokenEncrypted = strings.TrimSpace(input.RefreshTokenEncrypted)
	connection.TokenExpiresAt = input.TokenExpiresAt
	connection.LastRefreshAt = &now
	connection.ReauthorizationRequiredAt = nil
	connection.LastConnectionErrorCode = ""
	connection.LastConnectionErrorMessage = ""
	if err := connection.Update(service.db); err != nil {
		return nil, err
	}
	return connection, nil
}

func (service *CalendarConnectionService) MarkReauthorizationRequired(connectionID uint, errorCode string, errorMessage string) error {
	connection := &dbmodel.CalendarProviderConnection{}
	if err := service.db.Where("id = ?", connectionID).First(connection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCalendarConnectionNotFound
		}
		return err
	}
	now := service.now().UTC()
	if !canTransitionCalendarConnectionStatus(strings.TrimSpace(connection.Status), dbmodel.CalendarConnectionStatusReauthorizationNeeded) {
		return ErrCalendarConnectionNotFound
	}
	connection.Status = dbmodel.CalendarConnectionStatusReauthorizationNeeded
	connection.ReauthorizationRequiredAt = &now
	connection.LastConnectionErrorCode = strings.TrimSpace(errorCode)
	connection.LastConnectionErrorMessage = strings.TrimSpace(errorMessage)
	return connection.Update(service.db)
}

func (service *CalendarConnectionService) MarkDisconnected(connectionID uint) error {
	connection := &dbmodel.CalendarProviderConnection{}
	if err := service.db.Where("id = ?", connectionID).First(connection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCalendarConnectionNotFound
		}
		return err
	}
	if !canTransitionCalendarConnectionStatus(strings.TrimSpace(connection.Status), dbmodel.CalendarConnectionStatusDisconnected) {
		return ErrCalendarConnectionNotFound
	}
	connection.Status = dbmodel.CalendarConnectionStatusDisconnected
	return connection.Update(service.db)
}

func normalizeCalendarScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func canTransitionCalendarConnectionStatus(from string, to string) bool {
	if strings.TrimSpace(from) == "" {
		from = dbmodel.CalendarConnectionStatusActive
	}
	if from == to {
		return true
	}
	switch from {
	case dbmodel.CalendarConnectionStatusActive:
		return to == dbmodel.CalendarConnectionStatusReauthorizationNeeded || to == dbmodel.CalendarConnectionStatusDisconnected
	case dbmodel.CalendarConnectionStatusReauthorizationNeeded:
		return to == dbmodel.CalendarConnectionStatusActive || to == dbmodel.CalendarConnectionStatusDisconnected
	case dbmodel.CalendarConnectionStatusDisconnected:
		return to == dbmodel.CalendarConnectionStatusActive
	default:
		return false
	}
}
