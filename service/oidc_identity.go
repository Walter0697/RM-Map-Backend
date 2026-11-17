package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"strings"
)

func newOIDCSecureToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("secure entropy unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Existing accounts are explicitly bound by an administrator using verified
// provider issuer/subject values. Their user ID, role and relation data remain
// unchanged. Unbound identities fail closed, including username collisions.
func resolveVerifiedOIDCUser(issuer, subject, username string) (*dbmodel.User, error) {
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("missing verified identity")
	}
	var binding dbmodel.OIDCIdentity
	if err := database.Connection.Where("issuer = ? AND subject = ?", issuer, subject).First(&binding).Error; err != nil {
		return nil, err
	}
	var user dbmodel.User
	if err := database.Connection.First(&user, binding.UserID).Error; err != nil {
		return nil, err
	}
	if !user.IsActivated {
		return nil, fmt.Errorf("account disabled")
	}
	return &user, nil
}

// BindOIDCIdentity is for the operator-only bootstrap CLI, not an HTTP endpoint.
// The operator must independently verify the provider subject before calling it.
// It never overwrites a binding, changes roles, or activates an account.
func BindOIDCIdentity(userID uint, issuer, subject string) error {
	if userID == 0 || issuer == "" || subject == "" || strings.TrimSpace(issuer) != issuer || strings.TrimSpace(subject) != subject {
		return fmt.Errorf("exact issuer, subject and nonzero user ID required")
	}
	if issuer != config.Data.OIDC.Issuer {
		return fmt.Errorf("issuer must exactly match configured OIDC issuer")
	}
	return database.Connection.Transaction(func(tx *gorm.DB) error {
		var user dbmodel.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		if !user.IsActivated {
			return fmt.Errorf("account disabled")
		}
		var binding dbmodel.OIDCIdentity
		err := tx.Where("issuer = ? AND subject = ?", issuer, subject).First(&binding).Error
		if err == nil {
			if binding.UserID == userID {
				return nil
			}
			return fmt.Errorf("identity already belongs to another account")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&dbmodel.OIDCIdentity{Issuer: issuer, Subject: subject, UserID: userID}).Error
	})
}
