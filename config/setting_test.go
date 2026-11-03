package config

import (
	"os"
	"testing"
)

func TestApplyRedisEnvOverrides(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	t.Run("uses REDIS_DB when set", func(t *testing.T) {
		if err := os.Setenv("REDIS_DB", "5"); err != nil {
			t.Fatalf("failed to set REDIS_DB: %v", err)
		}
		defer os.Unsetenv("REDIS_DB")

		Data.Redis.DB = 0
		if err := applyRedisEnvOverrides(); err != nil {
			t.Fatalf("applyRedisEnvOverrides returned error: %v", err)
		}
		if Data.Redis.DB != 5 {
			t.Fatalf("expected redis db 5, got %d", Data.Redis.DB)
		}
	})

	t.Run("rejects non-integer REDIS_DB", func(t *testing.T) {
		if err := os.Setenv("REDIS_DB", "abc"); err != nil {
			t.Fatalf("failed to set REDIS_DB: %v", err)
		}
		defer os.Unsetenv("REDIS_DB")

		if err := applyRedisEnvOverrides(); err == nil {
			t.Fatalf("expected error for non-integer REDIS_DB")
		}
	})

	t.Run("rejects negative REDIS_DB", func(t *testing.T) {
		if err := os.Setenv("REDIS_DB", "-1"); err != nil {
			t.Fatalf("failed to set REDIS_DB: %v", err)
		}
		defer os.Unsetenv("REDIS_DB")

		if err := applyRedisEnvOverrides(); err == nil {
			t.Fatalf("expected error for negative REDIS_DB")
		}
	})

	t.Run("uses AUTH_SESSION_LIFETIME_SECONDS when set", func(t *testing.T) {
		if err := os.Setenv("AUTH_SESSION_LIFETIME_SECONDS", "31536000"); err != nil {
			t.Fatalf("failed to set AUTH_SESSION_LIFETIME_SECONDS: %v", err)
		}
		defer os.Unsetenv("AUTH_SESSION_LIFETIME_SECONDS")

		Data.App.AuthSessionLifetimeSeconds = 0
		if err := applyRedisEnvOverrides(); err != nil {
			t.Fatalf("applyRedisEnvOverrides returned error: %v", err)
		}
		if Data.App.AuthSessionLifetimeSeconds != 31536000 {
			t.Fatalf("expected app auth session lifetime 31536000, got %d", Data.App.AuthSessionLifetimeSeconds)
		}
	})

	t.Run("rejects non-positive AUTH_SESSION_LIFETIME_SECONDS", func(t *testing.T) {
		if err := os.Setenv("AUTH_SESSION_LIFETIME_SECONDS", "0"); err != nil {
			t.Fatalf("failed to set AUTH_SESSION_LIFETIME_SECONDS: %v", err)
		}
		defer os.Unsetenv("AUTH_SESSION_LIFETIME_SECONDS")

		if err := applyRedisEnvOverrides(); err == nil {
			t.Fatalf("expected error for non-positive AUTH_SESSION_LIFETIME_SECONDS")
		}
	})
}

func TestApplySocialMediaAnalysisEnvOverrides(t *testing.T) {
	original := Data
	defer func() { Data = original }()
	if err := os.Setenv("WCODEX_RUNNER_URL", "http://wcodex.test"); err != nil {
		t.Fatalf("failed to set WCODEX_RUNNER_URL: %v", err)
	}
	defer os.Unsetenv("WCODEX_RUNNER_URL")

	Data.SocialMediaAnalysis.RunnerURL = ""
	if err := applySocialMediaAnalysisEnvOverrides(); err != nil {
		t.Fatalf("applySocialMediaAnalysisEnvOverrides returned error: %v", err)
	}
	if Data.SocialMediaAnalysis.RunnerURL != "http://wcodex.test" {
		t.Fatalf("expected runner URL override, got %q", Data.SocialMediaAnalysis.RunnerURL)
	}
}

func TestApplyInfisicalEnvOverrides(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	for _, key := range []string{
		"INFISICAL_ENABLE",
		"INFISICAL_SITE_URL",
		"INFISICAL_PROJECT_ID",
		"INFISICAL_ENVIRONMENT",
		"INFISICAL_SECRET_PATH",
		"INFISICAL_CLIENT_ID",
		"INFISICAL_CLIENT_SECRET_ENV",
		"APIFY_TOKEN_ENV",
		"GEMINI_TOKEN_ENV",
		"APIFY_ACTOR_ID",
		"APIFY_BASE_URL",
	} {
		defer os.Unsetenv(key)
	}
	for key, value := range map[string]string{
		"INFISICAL_ENABLE":         "true",
		"INFISICAL_SITE_URL":       "https://secrets.example",
		"INFISICAL_PROJECT_ID":     "project-1",
		"INFISICAL_ENVIRONMENT":    "dev",
		"INFISICAL_SECRET_PATH":    "/roroadmap",
		"INFISICAL_CLIENT_ID":      "client-1",
		"INFISICAL_CLIENT_SECRET_ENV": "INFISICAL_TEST_CLIENT_SECRET",
		"APIFY_TOKEN_ENV":          "APIFY_TEST_TOKEN",
		"GEMINI_TOKEN_ENV":         "GEMINI_TEST_TOKEN",
		"APIFY_ACTOR_ID":           "actor-1",
		"APIFY_BASE_URL":           "https://api.apify.example",
	} {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}

	if err := applyInfisicalEnvOverrides(); err != nil {
		t.Fatalf("applyInfisicalEnvOverrides returned error: %v", err)
	}
	if !Data.Infisical.Enable || Data.Infisical.SiteURL != "https://secrets.example" ||
		Data.Infisical.ProjectID != "project-1" || Data.Infisical.SecretPath != "/roroadmap" ||
		Data.Infisical.ClientSecretEnv != "INFISICAL_TEST_CLIENT_SECRET" ||
		Data.Infisical.ApifyTokenEnv != "APIFY_TEST_TOKEN" || Data.Infisical.GeminiTokenEnv != "GEMINI_TEST_TOKEN" ||
		Data.Infisical.ApifyActorID != "actor-1" || Data.Infisical.ApifyBaseURL != "https://api.apify.example" {
		t.Fatalf("unexpected Infisical configuration: %+v", Data.Infisical)
	}
}

func TestApplyInfisicalEnvOverridesRejectsInvalidEnable(t *testing.T) {
	original := Data
	defer func() { Data = original }()
	if err := os.Setenv("INFISICAL_ENABLE", "sometimes"); err != nil {
		t.Fatalf("set INFISICAL_ENABLE: %v", err)
	}
	defer os.Unsetenv("INFISICAL_ENABLE")

	if err := applyInfisicalEnvOverrides(); err == nil {
		t.Fatalf("expected invalid Infisical enable value to fail")
	}
}
