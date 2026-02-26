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
}
