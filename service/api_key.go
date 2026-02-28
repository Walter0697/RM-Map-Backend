package service

import (
	"fmt"
	"mapmarker/backend/constant"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
	"sort"
	"strconv"
	"strings"
	"time"
)

const rawAPIKeyPrefix = "rmk_"

type APIKeyCreateInput struct {
	Name        string
	Scopes      []string
	RelationID  uint
	ActorUserID uint
	ExpiresAt   *time.Time
}

type APIKeyAuditEvent struct {
	Operation string
	SourceIP  string
	Success   bool
	Reason    string
}

type APIKeyUserOption struct {
	ID       uint
	Username string
	Role     string
}

type APIKeyRelationOption struct {
	ID         uint
	UserOneID  uint
	UserOne    string
	UserTwoID  uint
	UserTwo    string
	Display    string
}

func CreateAPIKey(input APIKeyCreateInput, operator *dbmodel.User) (*dbmodel.APIKey, string, error) {
	var relation dbmodel.UserRelation
	relation.ID = input.RelationID
	if err := relation.GetRelationById(database.Connection); err != nil {
		return nil, "", err
	}

	var actor dbmodel.User
	actor.ID = input.ActorUserID
	if err := actor.GetUserById(database.Connection); err != nil {
		return nil, "", err
	}

	secret := generateAPIKeySecret()
	keyHash, err := utils.GenerateHashedPassword(secret)
	if err != nil {
		return nil, "", err
	}

	normalizedScopes := normalizeScopes(input.Scopes)
	if len(normalizedScopes) == 0 {
		return nil, "", fmt.Errorf("at least one valid scope is required")
	}

	apiKey := &dbmodel.APIKey{
		Name:        strings.TrimSpace(input.Name),
		Prefix:      prefixSecret(secret),
		KeyHash:     keyHash,
		Scopes:      strings.Join(normalizedScopes, ","),
		Status:      dbmodel.APIKeyStatusActive,
		RelationID:  input.RelationID,
		ActorUserID: input.ActorUserID,
		ExpiresAt:   input.ExpiresAt,
	}

	if operator != nil {
		apiKey.CreatedBy = operator
		apiKey.UpdatedBy = operator
	}

	if err := apiKey.Create(database.Connection); err != nil {
		return nil, "", err
	}

	raw := BuildRawAPIKey(apiKey.ID, secret)
	return apiKey, raw, nil
}

func ListAPIKeys() ([]dbmodel.APIKey, error) {
	var keys []dbmodel.APIKey
	if err := database.Connection.Preload("Relation").Preload("ActorUser").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func ListAPIKeyOptions() ([]APIKeyUserOption, []APIKeyRelationOption, error) {
	users := make([]dbmodel.User, 0)
	if err := database.Connection.Order("username asc").Find(&users).Error; err != nil {
		return nil, nil, err
	}

	userOptions := make([]APIKeyUserOption, 0, len(users))
	for _, item := range users {
		userOptions = append(userOptions, APIKeyUserOption{
			ID:       item.ID,
			Username: item.Username,
			Role:     item.Role,
		})
	}

	relations := make([]dbmodel.UserRelation, 0)
	if err := database.Connection.Preload("UserOne").Preload("UserTwo").Order("id asc").Find(&relations).Error; err != nil {
		return nil, nil, err
	}

	relationOptions := make([]APIKeyRelationOption, 0, len(relations))
	for _, relation := range relations {
		relationOptions = append(relationOptions, APIKeyRelationOption{
			ID:         relation.ID,
			UserOneID:  relation.UserOneUID,
			UserOne:    relation.UserOne.Username,
			UserTwoID:  relation.UserTwoUID,
			UserTwo:    relation.UserTwo.Username,
			Display:    fmt.Sprintf("%s <-> %s", relation.UserOne.Username, relation.UserTwo.Username),
		})
	}

	return userOptions, relationOptions, nil
}

func RevokeAPIKey(id uint, operator *dbmodel.User) (*dbmodel.APIKey, error) {
	var apiKey dbmodel.APIKey
	apiKey.ID = id
	if err := apiKey.GetByID(database.Connection); err != nil {
		return nil, err
	}

	apiKey.Status = dbmodel.APIKeyStatusRevoked
	if operator != nil {
		apiKey.UpdatedBy = operator
	}

	if err := apiKey.Update(database.Connection); err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func RotateAPIKey(id uint, operator *dbmodel.User) (*dbmodel.APIKey, string, error) {
	var current dbmodel.APIKey
	current.ID = id
	if err := current.GetByID(database.Connection); err != nil {
		return nil, "", err
	}

	current.Status = dbmodel.APIKeyStatusRevoked
	if operator != nil {
		current.UpdatedBy = operator
	}
	if err := current.Update(database.Connection); err != nil {
		return nil, "", err
	}

	scopes := current.ScopeList()
	newKey, raw, err := CreateAPIKey(APIKeyCreateInput{
		Name:        current.Name,
		Scopes:      scopes,
		RelationID:  current.RelationID,
		ActorUserID: current.ActorUserID,
		ExpiresAt:   current.ExpiresAt,
	}, operator)
	if err != nil {
		return nil, "", err
	}

	newKey.RotatedFromID = &current.ID
	if operator != nil {
		newKey.UpdatedBy = operator
	}
	if err := newKey.Update(database.Connection); err != nil {
		return nil, "", err
	}

	return newKey, raw, nil
}

func DeleteAPIKey(id uint) error {
	var apiKey dbmodel.APIKey
	apiKey.ID = id
	if err := apiKey.GetByID(database.Connection); err != nil {
		return err
	}

	if err := database.Connection.Where("api_key_id = ?", apiKey.ID).Delete(&dbmodel.APIKeyAuditLog{}).Error; err != nil {
		return err
	}

	if err := database.Connection.Delete(&apiKey).Error; err != nil {
		return err
	}

	return nil
}

func AuthenticateAPIKey(raw string, event APIKeyAuditEvent) (*dbmodel.APIKey, error) {
	keyID, secret, err := ParseRawAPIKey(raw)
	if err != nil {
		_ = CreateAPIKeyAuditLog(nil, "unknown", event)
		return nil, &helper.APIKeyUnauthorizedError{}
	}

	var apiKey dbmodel.APIKey
	apiKey.ID = keyID
	if err := apiKey.GetByIDWithRelation(database.Connection); err != nil {
		_ = CreateAPIKeyAuditLog(&keyID, "unknown", event)
		return nil, &helper.APIKeyUnauthorizedError{}
	}

	if !apiKey.IsActive() || !utils.CompareHash(apiKey.KeyHash, secret) {
		_ = CreateAPIKeyAuditLog(&apiKey.ID, apiKey.Name, event)
		return nil, &helper.APIKeyUnauthorizedError{}
	}

	now := time.Now()
	apiKey.LastUsedAt = &now
	_ = apiKey.Update(database.Connection)

	successEvent := event
	successEvent.Success = true
	successEvent.Reason = ""
	_ = CreateAPIKeyAuditLog(&apiKey.ID, apiKey.Name, successEvent)

	return &apiKey, nil
}

func CreateAPIKeyAuditLog(apiKeyID *uint, apiKeyName string, event APIKeyAuditEvent) error {
	audit := dbmodel.APIKeyAuditLog{
		APIKeyID:   apiKeyID,
		APIKeyName: apiKeyName,
		SourceIP:   strings.TrimSpace(event.SourceIP),
		Operation:  strings.TrimSpace(event.Operation),
		Success:    event.Success,
		Reason:     strings.TrimSpace(event.Reason),
	}
	return audit.Create(database.Connection)
}

func CleanupOldAPIKeyData() error {
	now := time.Now()

	logCutoff := now.AddDate(0, 0, -config.Data.IntegrationAuth.LogRetentionDays)
	if err := database.Connection.Where("created_at < ?", logCutoff).Delete(&dbmodel.APIKeyAuditLog{}).Error; err != nil {
		return err
	}

	if config.Data.IntegrationAuth.MaxAuditLogRows > 0 {
		var count int64
		if err := database.Connection.Model(&dbmodel.APIKeyAuditLog{}).Count(&count).Error; err != nil {
			return err
		}

		if count > int64(config.Data.IntegrationAuth.MaxAuditLogRows) {
			excess := count - int64(config.Data.IntegrationAuth.MaxAuditLogRows)
			var ids []uint
			if err := database.Connection.Model(&dbmodel.APIKeyAuditLog{}).Order("created_at asc").Limit(int(excess)).Pluck("id", &ids).Error; err != nil {
				return err
			}
			if len(ids) > 0 {
				if err := database.Connection.Where("id IN ?", ids).Delete(&dbmodel.APIKeyAuditLog{}).Error; err != nil {
					return err
				}
			}
		}
	}

	keyCutoff := now.AddDate(0, 0, -config.Data.IntegrationAuth.RevokedKeyRetentionDay)
	if err := database.Connection.Where("status = ? AND updated_at < ?", dbmodel.APIKeyStatusRevoked, keyCutoff).Delete(&dbmodel.APIKey{}).Error; err != nil {
		return err
	}

	return nil
}

func HasScope(apiKey *dbmodel.APIKey, scope string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" || apiKey == nil {
		return false
	}

	for _, item := range apiKey.ScopeList() {
		if strings.TrimSpace(item) == scope {
			return true
		}
	}
	return false
}

func BuildRawAPIKey(id uint, secret string) string {
	return fmt.Sprintf("%s%d_%s", rawAPIKeyPrefix, id, secret)
}

func ParseRawAPIKey(raw string) (uint, string, error) {
	if !strings.HasPrefix(raw, rawAPIKeyPrefix) {
		return 0, "", fmt.Errorf("invalid key prefix")
	}

	payload := strings.TrimPrefix(raw, rawAPIKeyPrefix)
	parts := strings.SplitN(payload, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid key format")
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("invalid key id")
	}

	secret := strings.TrimSpace(parts[1])
	if secret == "" {
		return 0, "", fmt.Errorf("invalid key secret")
	}

	return uint(id), secret, nil
}

func normalizeScopes(scopes []string) []string {
	validScopes := map[string]struct{}{}
	for _, scope := range constant.AllAPIKeyScopes() {
		validScopes[strings.TrimSpace(scope)] = struct{}{}
	}

	unique := map[string]struct{}{}
	for _, scope := range scopes {
		value := strings.TrimSpace(scope)
		if value == "" {
			continue
		}
		if _, ok := validScopes[value]; !ok {
			continue
		}
		unique[value] = struct{}{}
	}

	result := make([]string, 0, len(unique))
	for scope := range unique {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result
}

func prefixSecret(secret string) string {
	trimmed := strings.TrimSpace(secret)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:8]
}

func generateAPIKeySecret() string {
	return fmt.Sprintf("%s%s%d", utils.GenerateLoginKey(), utils.GenerateLoginKey(), time.Now().UnixNano())
}
