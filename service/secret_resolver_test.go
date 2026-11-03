package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mapmarker/backend/config"
)

func TestSecretResolverPrefersInfisicalValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/auth/universal-auth/login":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accessToken":"access-token"}`))
		case r.URL.Path == "/api/v3/secrets/raw":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Fatalf("expected bearer token, got %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"secrets":[{"secretKey":"APIFY_TOKEN","secretValue":"infisical-token"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resolver := NewSecretResolver(config.InfisicalSetting{
		Enable:          true,
		SiteURL:         server.URL,
		ProjectID:       "project",
		Environment:     "dev",
		SecretPath:      "/roroadmap",
		ClientID:        "client",
		ClientSecret:    "client-secret",
		ApifyToken:      "toml-token",
		ApifyTokenEnv:   "APIFY_TEST_TOKEN",
	}, http.DefaultClient)

	t.Setenv("APIFY_TEST_TOKEN", "environment-token")
	value, err := resolver.Resolve(context.Background(), "APIFY_TOKEN")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if value != "infisical-token" {
		t.Fatalf("expected Infisical value, got %q", value)
	}
}

func TestSecretResolverFallsBackToEnvironmentThenToml(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		t.Setenv("APIFY_TEST_TOKEN", "environment-token")
		resolver := NewSecretResolver(config.InfisicalSetting{
			ApifyToken:    "toml-token",
			ApifyTokenEnv: "APIFY_TEST_TOKEN",
		}, http.DefaultClient)
		value, err := resolver.Resolve(context.Background(), "APIFY_TOKEN")
		if err != nil || value != "environment-token" {
			t.Fatalf("expected environment fallback, got %q, %v", value, err)
		}
	})

	t.Run("toml", func(t *testing.T) {
		resolver := NewSecretResolver(config.InfisicalSetting{ApifyToken: "toml-token"}, http.DefaultClient)
		value, err := resolver.Resolve(context.Background(), "APIFY_TOKEN")
		if err != nil || value != "toml-token" {
			t.Fatalf("expected TOML fallback, got %q, %v", value, err)
		}
	})
}

func TestSecretResolverRedactsProviderFailure(t *testing.T) {
	secret := "do-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider failed", http.StatusBadGateway)
	}))
	defer server.Close()

	resolver := NewSecretResolver(config.InfisicalSetting{
		Enable:       true,
		SiteURL:      server.URL,
		ClientID:     "client",
		ClientSecret: secret,
	}, http.DefaultClient)
	_, err := resolver.Resolve(context.Background(), "APIFY_TOKEN")
	if err == nil {
		t.Fatalf("expected provider error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposed secret: %v", err)
	}
}

func TestSecretResolverRequiresValue(t *testing.T) {
	resolver := NewSecretResolver(config.InfisicalSetting{}, http.DefaultClient)
	_, err := resolver.Resolve(context.Background(), "APIFY_TOKEN")
	if err == nil || !strings.Contains(err.Error(), "APIFY_TOKEN") {
		t.Fatalf("expected missing secret error, got %v", err)
	}
}
