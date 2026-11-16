package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
	"strings"
	"time"
)

const socialMediaShortcutSetupTokenLifetime = 10 * time.Minute

const socialMediaShortcutAPIKeySource = "social_media_shortcut"
const socialMediaShortcutAPIKeyName = "Roroadmap iOS Shortcut"

type SocialMediaShortcutSetupOwner struct {
	UserID     uint
	RelationID uint
}

var ErrInvalidSocialMediaShortcutSetupToken = errors.New("invalid or expired social media shortcut setup token")

func generateSocialMediaShortcutSetupToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func socialMediaShortcutSetupTokenExpiry(now time.Time) time.Time {
	return now.Add(socialMediaShortcutSetupTokenLifetime)
}

func CreateSocialMediaShortcutSetupToken(userID, relationID uint) (string, error) {
	if userID == 0 || relationID == 0 {
		return "", errors.New("user and relation are required")
	}
	plaintext, err := generateSocialMediaShortcutSetupToken()
	if err != nil {
		return "", err
	}
	hash, err := utils.GenerateHashedPassword(plaintext)
	if err != nil {
		return "", err
	}
	now := time.Now()
	setupToken := &dbmodel.SocialMediaShortcutSetupToken{
		TokenHash:  hash,
		UserID:     userID,
		RelationID: relationID,
		ExpiresAt:  socialMediaShortcutSetupTokenExpiry(now),
	}
	if err := database.Connection.Create(setupToken).Error; err != nil {
		return "", err
	}
	return plaintext, nil
}

func findSocialMediaShortcutSetupToken(plaintext string) (SocialMediaShortcutSetupOwner, uint, bool, error) {
	if strings.TrimSpace(plaintext) == "" {
		return SocialMediaShortcutSetupOwner{}, 0, false, nil
	}
	now := time.Now()
	var candidates []dbmodel.SocialMediaShortcutSetupToken
	if err := database.Connection.Where("expires_at > ?", now).Find(&candidates).Error; err != nil {
		return SocialMediaShortcutSetupOwner{}, 0, false, err
	}
	for _, candidate := range candidates {
		if !utils.CompareHash(candidate.TokenHash, plaintext) {
			continue
		}
		return SocialMediaShortcutSetupOwner{UserID: candidate.UserID, RelationID: candidate.RelationID}, candidate.ID, true, nil
	}
	return SocialMediaShortcutSetupOwner{}, 0, false, nil
}

func ValidateSocialMediaShortcutSetupToken(plaintext string) (bool, error) {
	_, _, found, err := findSocialMediaShortcutSetupToken(plaintext)
	return found, err
}

func ConsumeSocialMediaShortcutSetupToken(plaintext string) (SocialMediaShortcutSetupOwner, bool, error) {
	owner, tokenID, found, err := findSocialMediaShortcutSetupToken(plaintext)
	if err != nil || !found {
		return owner, found, err
	}
	result := database.Connection.Where("id = ? AND expires_at > ?", tokenID, time.Now()).Delete(&dbmodel.SocialMediaShortcutSetupToken{})
	if result.Error != nil {
		return SocialMediaShortcutSetupOwner{}, false, result.Error
	}
	if result.RowsAffected != 1 {
		return SocialMediaShortcutSetupOwner{}, false, nil
	}
	return owner, true, nil
}

func ExchangeSocialMediaShortcutSetupToken(token string) (string, error) {
	owner, found, err := ConsumeSocialMediaShortcutSetupToken(token)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrInvalidSocialMediaShortcutSetupToken
	}
	var actor dbmodel.User
	if err := database.Connection.Where("id = ?", owner.UserID).First(&actor).Error; err != nil {
		return "", err
	}
	_, rawKey, err := CreateAPIKey(APIKeyCreateInput{
		Name:        socialMediaShortcutAPIKeyName,
		Source:      socialMediaShortcutAPIKeySource,
		Scopes:      []string{constant.APIKeyScopeSocialMediaAnalysis},
		RelationID:  owner.RelationID,
		ActorUserID: &owner.UserID,
	}, &actor)
	if err != nil {
		return "", err
	}
	return rawKey, nil
}

func ListSocialMediaShortcutAPIKeys(userID uint) ([]dbmodel.APIKey, error) {
	var keys []dbmodel.APIKey
	if err := database.Connection.
		Where("actor_user_id = ?", userID).
		Where("source = ? OR (source = '' AND name = ?)", socialMediaShortcutAPIKeySource, socialMediaShortcutAPIKeyName).
		Order("created_at desc").
		Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func RevokeSocialMediaShortcutAPIKey(id, userID uint, operator *dbmodel.User) (*dbmodel.APIKey, error) {
	var key dbmodel.APIKey
	if err := database.Connection.
		Where("id = ? AND actor_user_id = ?", id, userID).
		Where("source = ? OR (source = '' AND name = ?)", socialMediaShortcutAPIKeySource, socialMediaShortcutAPIKeyName).
		First(&key).Error; err != nil {
		return nil, err
	}
	return RevokeAPIKey(id, operator)
}
