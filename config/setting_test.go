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
}
