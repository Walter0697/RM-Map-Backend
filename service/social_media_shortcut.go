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

func ConsumeSocialMediaShortcutSetupToken(plaintext string) (SocialMediaShortcutSetupOwner, bool, error) {
	if strings.TrimSpace(plaintext) == "" {
		return SocialMediaShortcutSetupOwner{}, false, nil
	}
	now := time.Now()
	var candidates []dbmodel.SocialMediaShortcutSetupToken
	if err := database.Connection.Where("expires_at > ?", now).Find(&candidates).Error; err != nil {
		return SocialMediaShortcutSetupOwner{}, false, err
	}
	for _, candidate := range candidates {
		if !utils.CompareHash(candidate.TokenHash, plaintext) {
			continue
		}
		result := database.Connection.Where("id = ? AND expires_at > ?", candidate.ID, now).Delete(&dbmodel.SocialMediaShortcutSetupToken{})
		if result.Error != nil {
			return SocialMediaShortcutSetupOwner{}, false, result.Error
		}
		if result.RowsAffected != 1 {
			return SocialMediaShortcutSetupOwner{}, false, nil
		}
		return SocialMediaShortcutSetupOwner{UserID: candidate.UserID, RelationID: candidate.RelationID}, true, nil
	}
	return SocialMediaShortcutSetupOwner{}, false, nil
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
		Name:        "Roroadmap iOS Shortcut",
		Scopes:      []string{constant.APIKeyScopeSocialMediaAnalysis},
		RelationID:  owner.RelationID,
		ActorUserID: &owner.UserID,
	}, &actor)
	if err != nil {
		return "", err
	}
	return rawKey, nil
}
