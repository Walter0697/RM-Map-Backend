package config

import (
	"fmt"
	"strings"
)

const (
	AuthStateModeDualWrite    = "dual-write"
	AuthStateModeRedisPrimary = "redis-primary"
	AuthStateModePostgresOff  = "postgres-off"
)

func ValidateAuthStateConfig() error {
	applyAuthStateDefaults()

	mode := strings.ToLower(strings.TrimSpace(Data.AuthState.MigrationMode))
	switch mode {
	case AuthStateModeDualWrite, AuthStateModeRedisPrimary, AuthStateModePostgresOff:
	default:
		return fmt.Errorf("unsupported [authstate].migrationmode %q", Data.AuthState.MigrationMode)
	}
	Data.AuthState.MigrationMode = mode

	if strings.TrimSpace(Data.AuthState.KeyPrefix) == "" {
		return fmt.Errorf("[authstate].keyprefix cannot be empty")
	}
	if Data.AuthState.SessionTTLSeconds <= 0 {
		return fmt.Errorf("[authstate].sessionttlseconds must be > 0")
	}
	if Data.AuthState.RedisDialTimeoutMS <= 0 || Data.AuthState.RedisReadTimeoutMS <= 0 || Data.AuthState.RedisWriteTimeoutMS <= 0 {
		return fmt.Errorf("redis timeout values must be > 0")
	}

	if mode == AuthStateModeRedisPrimary || mode == AuthStateModePostgresOff {
		if !Data.Redis.Enable {
			return fmt.Errorf("redis must be enabled for authstate migration mode %q", mode)
		}
	}

	return nil
}

func applyAuthStateDefaults() {
	if strings.TrimSpace(Data.AuthState.MigrationMode) == "" {
		Data.AuthState.MigrationMode = AuthStateModeDualWrite
	}
	if strings.TrimSpace(Data.AuthState.KeyPrefix) == "" {
		Data.AuthState.KeyPrefix = "auth:v1"
	}
	if Data.AuthState.SessionTTLSeconds <= 0 {
		if Data.App.AuthSessionLifetimeSeconds > 0 {
			Data.AuthState.SessionTTLSeconds = Data.App.AuthSessionLifetimeSeconds
		} else {
			Data.AuthState.SessionTTLSeconds = DefaultAuthSessionLifetimeSeconds
		}
	}
	if Data.AuthState.RedisDialTimeoutMS <= 0 {
		Data.AuthState.RedisDialTimeoutMS = 1500
	}
	if Data.AuthState.RedisReadTimeoutMS <= 0 {
		Data.AuthState.RedisReadTimeoutMS = 1500
	}
	if Data.AuthState.RedisWriteTimeoutMS <= 0 {
		Data.AuthState.RedisWriteTimeoutMS = 1500
	}

	if strings.TrimSpace(Data.Redis.Host) == "" {
		Data.Redis.Host = "localhost"
	}
	if strings.TrimSpace(Data.Redis.Port) == "" {
		Data.Redis.Port = "6379"
	}
}
