package config

import "testing"

func TestResolveAuthMode(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	Data = Config{
		App: AppEnv{Environment: "development"},
		OIDC: OIDCSetting{
			Enable: true,
		},
	}

	mode, err := ResolveAuthMode()
	if err != nil {
		t.Fatalf("ResolveAuthMode returned error: %v", err)
	}
	if mode != AuthModeOIDC {
		t.Fatalf("expected mode %s, got %s", AuthModeOIDC, mode)
	}

	Data.App.AuthMode = AuthModeLocalPassword
	mode, err = ResolveAuthMode()
	if err != nil {
		t.Fatalf("expected local-password in development, got error: %v", err)
	}
	if mode != AuthModeLocalPassword {
		t.Fatalf("expected mode %s, got %s", AuthModeLocalPassword, mode)
	}

	Data.App.Environment = "production"
	_, err = ResolveAuthMode()
	if err == nil {
		t.Fatalf("expected error for local-password in production")
	}
}

func TestValidateAuthConfigDefaults(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	Data = Config{
		App: AppEnv{
			Environment: "production",
			AuthMode:    AuthModeOIDC,
		},
		OIDC: OIDCSetting{
			Enable:       true,
			Issuer:       "http://localhost:9000/application/o/rmmap/",
			ClientID:     "id",
			ClientSecret: "secret",
			RedirectURL:  "http://localhost:1998/auth/oidc/callback",
		},
		Redis: RedisSetting{
			Enable: true,
		},
	}

	if err := ValidateAuthConfig(); err != nil {
		t.Fatalf("ValidateAuthConfig returned error: %v", err)
	}

	if Data.OIDC.DefaultRole == "" {
		t.Fatalf("expected default role to be set")
	}
	if len(Data.OIDC.Scopes) == 0 {
		t.Fatalf("expected default scopes to be set")
	}
	if Data.OIDC.UsernameClaim == "" {
		t.Fatalf("expected default username claim to be set")
	}
	if Data.IntegrationAuth.LogRetentionDays <= 0 {
		t.Fatalf("expected positive log retention default")
	}
	if Data.IntegrationAuth.MaxAuditLogRows <= 0 {
		t.Fatalf("expected positive max audit log rows default")
	}
	if Data.IntegrationAuth.RevokedKeyRetentionDay <= 0 {
		t.Fatalf("expected positive revoked key retention default")
	}
	if Data.IntegrationAuth.CleanupIntervalHours <= 0 {
		t.Fatalf("expected positive cleanup interval default")
	}
	if Data.AuthState.MigrationMode == "" {
		t.Fatalf("expected auth state migration mode default")
	}
	if Data.AuthState.SessionTTLSeconds <= 0 {
		t.Fatalf("expected positive auth state session ttl")
	}
	if Data.App.AuthSessionLifetimeSeconds != DefaultAuthSessionLifetimeSeconds {
		t.Fatalf("expected auth session lifetime default %d, got %d", DefaultAuthSessionLifetimeSeconds, Data.App.AuthSessionLifetimeSeconds)
	}
	if Data.AuthState.SessionTTLSeconds != Data.App.AuthSessionLifetimeSeconds {
		t.Fatalf("expected auth state session ttl to align with auth session lifetime")
	}
	if Data.AuthState.KeyPrefix == "" {
		t.Fatalf("expected auth state key prefix default")
	}
}

func TestValidateAuthStateConfigModeValidation(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	Data = Config{
		Redis: RedisSetting{
			Enable: true,
		},
		AuthState: AuthStateSetting{
			MigrationMode:       AuthStateModeRedisPrimary,
			KeyPrefix:           "auth:v1",
			SessionTTLSeconds:   3600,
			RedisDialTimeoutMS:  1000,
			RedisReadTimeoutMS:  1000,
			RedisWriteTimeoutMS: 1000,
		},
	}
	if err := ValidateAuthStateConfig(); err != nil {
		t.Fatalf("ValidateAuthStateConfig returned error: %v", err)
	}

	Data.AuthState.MigrationMode = "unknown"
	if err := ValidateAuthStateConfig(); err == nil {
		t.Fatalf("expected mode validation error")
	}

	Data.AuthState.MigrationMode = AuthStateModePostgresOff
	Data.Redis.Enable = false
	if err := ValidateAuthStateConfig(); err == nil {
		t.Fatalf("expected redis enabled validation error")
	}
}

func TestAuthLifetimeAlignmentStatus(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	Data = Config{
		App: AppEnv{
			AuthSessionLifetimeSeconds: 31536000,
		},
		AuthState: AuthStateSetting{
			SessionTTLSeconds: 31536000,
		},
		OIDC: OIDCSetting{
			SessionLifetimeSeconds:      31536000,
			AccessTokenLifetimeSeconds:  31536000,
			RefreshTokenLifetimeSeconds: 31536000,
		},
	}

	alignment := AuthLifetimeAlignmentStatus()
	if !alignment.AuthStateAligned {
		t.Fatalf("expected auth state alignment")
	}
	if !alignment.OIDCAligned {
		t.Fatalf("expected oidc alignment")
	}

	Data.AuthState.SessionTTLSeconds = 3600
	alignment = AuthLifetimeAlignmentStatus()
	if alignment.AuthStateAligned {
		t.Fatalf("expected auth state mismatch")
	}
}
