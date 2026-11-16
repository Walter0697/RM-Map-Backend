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

func TestApplyGoogleMapsDefaults(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	Data.GoogleMaps = GoogleMapsSetting{}
	applyGoogleMapsDefaults()
	if Data.GoogleMaps.Enable {
		t.Fatal("expected Google Maps to be disabled by default")
	}
	if Data.GoogleMaps.BaseURL != "https://places.googleapis.com" {
		t.Fatalf("expected default Google Maps base URL, got %q", Data.GoogleMaps.BaseURL)
	}
	if Data.GoogleMaps.TimeoutMS != 5000 || Data.GoogleMaps.MaxResults != 5 {
		t.Fatalf("unexpected Google Maps defaults: %+v", Data.GoogleMaps)
	}
}

func TestApplyObservabilityDefaults(t *testing.T) {
	original := Data
	defer func() { Data = original }()

	Data.Phoenix = PhoenixSetting{}
	Data.MinIO = MinIOSetting{}
	applyObservabilityDefaults()

	if Data.Phoenix.Endpoint != "http://phoenix.home" || Data.Phoenix.Project != "roroadmap-social-analysis" {
		t.Fatalf("unexpected Phoenix defaults: %+v", Data.Phoenix)
	}
	if Data.Phoenix.TimeoutMS != 5000 || Data.Phoenix.BatchSize != 64 {
		t.Fatalf("unexpected Phoenix limits: %+v", Data.Phoenix)
	}
	if Data.MinIO.Region != "us-east-1" || Data.MinIO.Bucket != "roroadmap-social-analysis" {
		t.Fatalf("unexpected MinIO defaults: %+v", Data.MinIO)
	}
	if Data.MinIO.TimeoutMS != 10000 || Data.MinIO.MaxBytes != 25*1024*1024 {
		t.Fatalf("unexpected MinIO limits: %+v", Data.MinIO)
	}
}

func TestApplyObservabilityEnvOverrides(t *testing.T) {
	original := Data
	defer func() { Data = original }()
	for key, value := range map[string]string{
		"PHOENIX_ENABLE":       "true",
		"PHOENIX_ENDPOINT":     "https://phoenix.example",
		"PHOENIX_PROJECT":      "social-analysis-dev",
		"PHOENIX_API_KEY_ENV":  "PHOENIX_TEST_KEY",
		"MINIO_ENABLE":         "true",
		"MINIO_ENDPOINT":       "minio.example:9000",
		"MINIO_BUCKET":         "analysis-dev",
		"MINIO_ACCESS_KEY_ENV": "MINIO_TEST_ACCESS",
		"MINIO_SECRET_KEY_ENV": "MINIO_TEST_SECRET",
	} {
		t.Setenv(key, value)
	}
	if err := applyObservabilityEnvOverrides(); err != nil {
		t.Fatalf("applyObservabilityEnvOverrides returned error: %v", err)
	}
	if !Data.Phoenix.Enable || Data.Phoenix.Endpoint != "https://phoenix.example" || Data.Phoenix.Project != "social-analysis-dev" || Data.Phoenix.APIKeyEnv != "PHOENIX_TEST_KEY" {
		t.Fatalf("unexpected Phoenix overrides: %+v", Data.Phoenix)
	}
	if !Data.MinIO.Enable || Data.MinIO.Endpoint != "minio.example:9000" || Data.MinIO.Bucket != "analysis-dev" || Data.MinIO.AccessKeyEnv != "MINIO_TEST_ACCESS" || Data.MinIO.SecretKeyEnv != "MINIO_TEST_SECRET" {
		t.Fatalf("unexpected MinIO overrides: %+v", Data.MinIO)
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
		"INFISICAL_ENABLE":            "true",
		"INFISICAL_SITE_URL":          "https://secrets.example",
		"INFISICAL_PROJECT_ID":        "project-1",
		"INFISICAL_ENVIRONMENT":       "dev",
		"INFISICAL_SECRET_PATH":       "/roroadmap",
		"INFISICAL_CLIENT_ID":         "client-1",
		"INFISICAL_CLIENT_SECRET_ENV": "INFISICAL_TEST_CLIENT_SECRET",
		"APIFY_TOKEN_ENV":             "APIFY_TEST_TOKEN",
		"GEMINI_TOKEN_ENV":            "GEMINI_TEST_TOKEN",
		"APIFY_ACTOR_ID":              "actor-1",
		"APIFY_BASE_URL":              "https://api.apify.example",
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
