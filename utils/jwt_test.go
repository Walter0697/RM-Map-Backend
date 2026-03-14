package utils

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"mapmarker/backend/config"
)

func TestGenerateTokenUsesConfiguredTokenLifetime(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()

	config.Data = config.Config{
		App:  config.AppEnv{AuthSessionLifetimeSeconds: 3600, JWT: "test-jwt-secret"},
		OIDC: config.OIDCSetting{Enable: true, SessionLifetimeSeconds: 7200},
	}

	token, err := GenerateToken("alice", "session-secret")
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Data.App.JWT), nil
	})
	if err != nil {
		t.Fatalf("unable to parse generated token for assertion: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("expected map claims")
	}
	if username, _ := claims["username"].(string); username != "alice" {
		t.Fatalf("expected username alice, got %q", username)
	}
	if secret, _ := claims["secret"].(string); secret != "session-secret" {
		t.Fatalf("expected secret session-secret, got %q", secret)
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("expected numeric exp claim")
	}

	now := time.Now().UTC().Unix()
	expAt := int64(exp)
	if expAt < now+7190 || expAt > now+7210 {
		t.Fatalf("expected exp around now+7200, got %d", expAt)
	}
}

func TestParseTokenRejectsExpiredToken(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()

	config.Data = config.Config{App: config.AppEnv{JWT: "test-jwt-secret"}}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": "alice",
		"secret":   "session-secret",
		"iat":      time.Now().UTC().Add(-time.Minute).Unix(),
		"exp":      time.Now().UTC().Add(-time.Minute).Unix(),
	})
	expired, err := token.SignedString([]byte(config.Data.App.JWT))
	if err != nil {
		t.Fatalf("unable to build expired token: %v", err)
	}

	if _, err := ParseToken(expired); err == nil {
		t.Fatalf("expected parse failure for expired token")
	}
}
