package config

import (
	"fmt"
	"log"
	"strings"
)

const (
	AuthModeOIDC          = "oidc"
	AuthModeLocalPassword = "local-password"
)

func IsLocalEnvironment(environment string) bool {
	env := strings.ToLower(strings.TrimSpace(environment))
	return env == "development" || env == "local" || env == "test"
}

func ResolveAuthMode() (string, error) {
	mode := strings.ToLower(strings.TrimSpace(Data.App.AuthMode))

	if mode == "" {
		if Data.OIDC.Enable {
			mode = AuthModeOIDC
		} else {
			mode = AuthModeLocalPassword
		}
	}

	switch mode {
	case AuthModeOIDC:
		if !Data.OIDC.Enable {
			return "", fmt.Errorf("auth mode oidc selected but [oidc].enable=false")
		}
		if Data.LDAP.Enable {
			log.Printf("Deprecated config: [ldap] is enabled but ignored in oidc mode")
		}
		return AuthModeOIDC, nil
	case AuthModeLocalPassword:
		if !IsLocalEnvironment(Data.App.Environment) {
			return "", fmt.Errorf("local-password mode is only allowed in local/development/test environments")
		}
		if Data.LDAP.Enable {
			log.Printf("Deprecated config: [ldap] is enabled but ignored in local-password mode")
		}
		return AuthModeLocalPassword, nil
	default:
		return "", fmt.Errorf("unsupported app authmode '%s', expected '%s' or '%s'", Data.App.AuthMode, AuthModeOIDC, AuthModeLocalPassword)
	}
}

func ValidateAuthConfig() error {
	applyIntegrationDefaults()

	mode, err := ResolveAuthMode()
	if err != nil {
		return err
	}

	if mode != AuthModeOIDC {
		return nil
	}

	if strings.TrimSpace(Data.OIDC.Issuer) == "" {
		return fmt.Errorf("missing [oidc].issuer")
	}
	if strings.TrimSpace(Data.OIDC.ClientID) == "" {
		return fmt.Errorf("missing [oidc].clientid")
	}
	if strings.TrimSpace(Data.OIDC.ClientSecret) == "" {
		return fmt.Errorf("missing [oidc].clientsecret")
	}
	if strings.TrimSpace(Data.OIDC.RedirectURL) == "" {
		return fmt.Errorf("missing [oidc].redirecturl")
	}

	if strings.TrimSpace(Data.OIDC.DefaultRole) == "" {
		Data.OIDC.DefaultRole = "user"
	}
	if len(Data.OIDC.Scopes) == 0 {
		Data.OIDC.Scopes = []string{"openid", "profile", "email"}
	}
	if strings.TrimSpace(Data.OIDC.UsernameClaim) == "" {
		Data.OIDC.UsernameClaim = "preferred_username"
	}

	return nil
}

func applyIntegrationDefaults() {
	if Data.IntegrationAuth.LogRetentionDays <= 0 {
		Data.IntegrationAuth.LogRetentionDays = 30
	}
	if Data.IntegrationAuth.MaxAuditLogRows <= 0 {
		Data.IntegrationAuth.MaxAuditLogRows = 200000
	}
	if Data.IntegrationAuth.RevokedKeyRetentionDay <= 0 {
		Data.IntegrationAuth.RevokedKeyRetentionDay = 30
	}
	if Data.IntegrationAuth.CleanupIntervalHours <= 0 {
		Data.IntegrationAuth.CleanupIntervalHours = 24
	}
}
