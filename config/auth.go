package config

import (
	"fmt"
	"log"
	"strings"
)

const (
	AuthModeOIDC                      = "oidc"
	AuthModeLocalPassword             = "local-password"
	DefaultAuthSessionLifetimeSeconds = 60 * 60 * 24 * 365
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
	applyAuthSessionDefaults()
	applyIntegrationDefaults()
	applyCalendarDefaults()
	if err := ValidateAuthStateConfig(); err != nil {
		return err
	}

	mode, err := ResolveAuthMode()
	if err != nil {
		return err
	}

	if mode != AuthModeOIDC {
		logAuthLifetimeAlignment()
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

	logAuthLifetimeAlignment()
	return nil
}

type AuthLifetimeAlignment struct {
	ConfiguredSessionTTLSeconds int  `json:"configuredSessionTTLSeconds"`
	AuthStateSessionTTLSeconds  int  `json:"authStateSessionTTLSeconds"`
	OIDCSessionTTLSeconds       int  `json:"oidcSessionTTLSeconds"`
	OIDCAccessTokenTTLSeconds   int  `json:"oidcAccessTokenTTLSeconds"`
	OIDCRefreshTokenTTLSeconds  int  `json:"oidcRefreshTokenTTLSeconds"`
	AuthStateAligned            bool `json:"authStateAligned"`
	OIDCAligned                 bool `json:"oidcAligned"`
}

func AuthLifetimeAlignmentStatus() AuthLifetimeAlignment {
	configuredTTL := Data.App.AuthSessionLifetimeSeconds
	if configuredTTL <= 0 {
		configuredTTL = DefaultAuthSessionLifetimeSeconds
	}

	authStateAligned := Data.AuthState.SessionTTLSeconds == configuredTTL

	oidcSession := Data.OIDC.SessionLifetimeSeconds
	oidcAccess := Data.OIDC.AccessTokenLifetimeSeconds
	oidcRefresh := Data.OIDC.RefreshTokenLifetimeSeconds

	oidcAligned := true
	if oidcSession > 0 && oidcSession != configuredTTL {
		oidcAligned = false
	}
	if oidcAccess > 0 && oidcAccess != configuredTTL {
		oidcAligned = false
	}
	if oidcRefresh > 0 && oidcRefresh != configuredTTL {
		oidcAligned = false
	}

	return AuthLifetimeAlignment{
		ConfiguredSessionTTLSeconds: configuredTTL,
		AuthStateSessionTTLSeconds:  Data.AuthState.SessionTTLSeconds,
		OIDCSessionTTLSeconds:       oidcSession,
		OIDCAccessTokenTTLSeconds:   oidcAccess,
		OIDCRefreshTokenTTLSeconds:  oidcRefresh,
		AuthStateAligned:            authStateAligned,
		OIDCAligned:                 oidcAligned,
	}
}

func applyAuthSessionDefaults() {
	if Data.App.AuthSessionLifetimeSeconds <= 0 {
		Data.App.AuthSessionLifetimeSeconds = DefaultAuthSessionLifetimeSeconds
	}
}

func logAuthLifetimeAlignment() {
	alignment := AuthLifetimeAlignmentStatus()
	if alignment.AuthStateAligned && alignment.OIDCAligned {
		log.Printf("Auth lifetime policy aligned (session=%ds)", alignment.ConfiguredSessionTTLSeconds)
		return
	}

	log.Printf(
		"Auth lifetime policy mismatch detected: app.authsessionttlseconds=%d authstate.sessionttlseconds=%d oidc.sessionttlseconds=%d oidc.accesstokenttlseconds=%d oidc.refreshtokenttlseconds=%d",
		alignment.ConfiguredSessionTTLSeconds,
		alignment.AuthStateSessionTTLSeconds,
		alignment.OIDCSessionTTLSeconds,
		alignment.OIDCAccessTokenTTLSeconds,
		alignment.OIDCRefreshTokenTTLSeconds,
	)
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

func applyCalendarDefaults() {
	if strings.TrimSpace(Data.CalendarGoogle.AuthEndpoint) == "" {
		Data.CalendarGoogle.AuthEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if strings.TrimSpace(Data.CalendarGoogle.TokenEndpoint) == "" {
		Data.CalendarGoogle.TokenEndpoint = "https://oauth2.googleapis.com/token"
	}
	if strings.TrimSpace(Data.CalendarGoogle.APIBaseURL) == "" {
		Data.CalendarGoogle.APIBaseURL = "https://www.googleapis.com/calendar/v3"
	}
	if len(Data.CalendarGoogle.Scopes) == 0 {
		Data.CalendarGoogle.Scopes = []string{
			"https://www.googleapis.com/auth/calendar.events",
		}
	}
}
