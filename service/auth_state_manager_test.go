package service

import (
	"errors"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"testing"
	"time"
)

type fakeRedisClient struct {
	store      map[string]string
	getErr     error
	setErr     error
	delErr     error
	pingErr    error
	lastSetTTL time.Duration
}

func (f *fakeRedisClient) Ping() error {
	return f.pingErr
}

func (f *fakeRedisClient) Get(key string) (string, bool, error) {
	if f.getErr != nil {
		return "", false, f.getErr
	}
	value, ok := f.store[key]
	return value, ok, nil
}

func (f *fakeRedisClient) SetEX(key string, value string, ttl time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.store[key] = value
	f.lastSetTTL = ttl
	return nil
}

func (f *fakeRedisClient) Del(key string) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.store, key)
	return nil
}

func TestAuthStateValidateRedisPrimaryFallbackBackfill(t *testing.T) {
	redis := &fakeRedisClient{store: map[string]string{}}
	manager := &defaultAuthStateManager{
		mode:         config.AuthStateModeRedisPrimary,
		keyPrefix:    "auth:v1",
		sessionTTL:   time.Hour,
		redisEnabled: true,
		redisClient:  redis,
		validatePostgresFn: func(username string, secret string) (bool, error) {
			return username == "alice" && secret == "secret-1", nil
		},
		writePostgresFn:  func(user *dbmodel.User, secret string) error { return nil },
		revokePostgresFn: func(user *dbmodel.User) error { return nil },
	}

	valid, err := manager.Validate("alice", "secret-1")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !valid {
		t.Fatalf("expected validation success")
	}
	if manager.metrics.validateFallbackCount.Load() != 1 {
		t.Fatalf("expected fallback count 1, got %d", manager.metrics.validateFallbackCount.Load())
	}
	if _, exists := redis.store[manager.sessionKey("alice")]; !exists {
		t.Fatalf("expected redis backfill to persist session key")
	}
}

func TestAuthStateValidatePostgresOffReturnsUnavailableOnRedisError(t *testing.T) {
	redis := &fakeRedisClient{
		store:  map[string]string{},
		getErr: errors.New("connection reset"),
	}
	manager := &defaultAuthStateManager{
		mode:            config.AuthStateModePostgresOff,
		keyPrefix:       "auth:v1",
		sessionTTL:      time.Hour,
		redisEnabled:    true,
		redisClient:     redis,
		writePostgresFn: func(user *dbmodel.User, secret string) error { return nil },
		validatePostgresFn: func(username string, secret string) (bool, error) {
			return false, nil
		},
		revokePostgresFn: func(user *dbmodel.User) error { return nil },
	}

	valid, err := manager.Validate("alice", "secret-1")
	if err == nil {
		t.Fatalf("expected auth-state-unavailable error")
	}
	var unavailable *AuthStateUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected AuthStateUnavailableError, got %T", err)
	}
	if valid {
		t.Fatalf("expected validation failure when redis is unavailable")
	}
}

func TestAuthStateIssueDualWriteWritesBothStores(t *testing.T) {
	redis := &fakeRedisClient{store: map[string]string{}}
	wrotePostgres := false
	manager := &defaultAuthStateManager{
		mode:         config.AuthStateModeDualWrite,
		keyPrefix:    "auth:v1",
		sessionTTL:   time.Hour,
		redisEnabled: true,
		redisClient:  redis,
		writePostgresFn: func(user *dbmodel.User, secret string) error {
			wrotePostgres = (user.Username == "alice" && secret == "secret-1")
			return nil
		},
		validatePostgresFn: func(username string, secret string) (bool, error) { return false, nil },
		revokePostgresFn:   func(user *dbmodel.User) error { return nil },
	}

	user := &dbmodel.User{Username: "alice"}
	if err := manager.Issue(user, "secret-1"); err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if !wrotePostgres {
		t.Fatalf("expected postgres write")
	}
	if _, exists := redis.store[manager.sessionKey("alice")]; !exists {
		t.Fatalf("expected redis write")
	}
	if redis.lastSetTTL != time.Hour {
		t.Fatalf("expected redis ttl 1h, got %s", redis.lastSetTTL)
	}
}

func TestAuthStateRevokeRedisPrimaryRemovesSession(t *testing.T) {
	redis := &fakeRedisClient{store: map[string]string{}}
	manager := &defaultAuthStateManager{
		mode:               config.AuthStateModeRedisPrimary,
		keyPrefix:          "auth:v1",
		sessionTTL:         365 * 24 * time.Hour,
		redisEnabled:       true,
		redisClient:        redis,
		writePostgresFn:    func(user *dbmodel.User, secret string) error { return nil },
		validatePostgresFn: func(username string, secret string) (bool, error) { return false, nil },
		revokePostgresFn:   func(user *dbmodel.User) error { return nil },
	}

	user := &dbmodel.User{Username: "alice"}
	if err := manager.Issue(user, "secret-1"); err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if _, exists := redis.store[manager.sessionKey("alice")]; !exists {
		t.Fatalf("expected redis session to exist before revoke")
	}

	if err := manager.Revoke(user); err != nil {
		t.Fatalf("Revoke returned error: %v", err)
	}
	if _, exists := redis.store[manager.sessionKey("alice")]; exists {
		t.Fatalf("expected redis session to be removed after revoke")
	}
}
