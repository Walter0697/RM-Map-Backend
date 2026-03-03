package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type AuthStateUnavailableError struct {
	Cause error
}

func (e *AuthStateUnavailableError) Error() string {
	if e.Cause == nil {
		return "auth state unavailable"
	}
	return fmt.Sprintf("auth state unavailable: %v", e.Cause)
}

func (e *AuthStateUnavailableError) Unwrap() error {
	return e.Cause
}

type authStateMetricsSnapshot struct {
	ValidateCount          uint64 `json:"validateCount"`
	ValidateFallbackCount  uint64 `json:"validateFallbackCount"`
	ValidateErrorCount     uint64 `json:"validateErrorCount"`
	ValidateLatencyTotalMS uint64 `json:"validateLatencyTotalMs"`
	RevocationErrorCount   uint64 `json:"revocationErrorCount"`
}

type authStateMetrics struct {
	validateCount          atomic.Uint64
	validateFallbackCount  atomic.Uint64
	validateErrorCount     atomic.Uint64
	validateLatencyTotalMS atomic.Uint64
	revocationErrorCount   atomic.Uint64
}

func (m *authStateMetrics) snapshot() authStateMetricsSnapshot {
	return authStateMetricsSnapshot{
		ValidateCount:          m.validateCount.Load(),
		ValidateFallbackCount:  m.validateFallbackCount.Load(),
		ValidateErrorCount:     m.validateErrorCount.Load(),
		ValidateLatencyTotalMS: m.validateLatencyTotalMS.Load(),
		RevocationErrorCount:   m.revocationErrorCount.Load(),
	}
}

type authStateManager interface {
	Issue(user *dbmodel.User, secret string) error
	Validate(username string, secret string) (bool, error)
	Revoke(user *dbmodel.User) error
	Metrics() authStateMetricsSnapshot
}

type authStateRecord struct {
	SecretHash string `json:"secretHash"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type authStateLookup int

const (
	authStateLookupMiss authStateLookup = iota
	authStateLookupValid
	authStateLookupInvalid
)

type defaultAuthStateManager struct {
	mode               string
	keyPrefix          string
	sessionTTL         time.Duration
	redisEnabled       bool
	redisClient        redisClient
	metrics            authStateMetrics
	writePostgresFn    func(user *dbmodel.User, secret string) error
	validatePostgresFn func(username string, secret string) (bool, error)
	revokePostgresFn   func(user *dbmodel.User) error
}

var (
	authStateManagerOnce sync.Once
	globalAuthStateMgr   authStateManager
)

func InitAuthStateManager() {
	authStateManagerOnce = sync.Once{}
	globalAuthStateMgr = nil
	getAuthStateManager()
}

func setAuthStateManagerForTest(mgr authStateManager) func() {
	original := globalAuthStateMgr
	globalAuthStateMgr = mgr
	return func() {
		globalAuthStateMgr = original
	}
}

func AuthStateMetrics() authStateMetricsSnapshot {
	return getAuthStateManager().Metrics()
}

func getAuthStateManager() authStateManager {
	if globalAuthStateMgr != nil {
		return globalAuthStateMgr
	}

	authStateManagerOnce.Do(func() {
		mgr := &defaultAuthStateManager{
			mode:         strings.TrimSpace(config.Data.AuthState.MigrationMode),
			keyPrefix:    strings.TrimSpace(config.Data.AuthState.KeyPrefix),
			sessionTTL:   time.Duration(config.Data.AuthState.SessionTTLSeconds) * time.Second,
			redisEnabled: config.Data.Redis.Enable,
		}
		if mgr.redisEnabled {
			mgr.redisClient = newRawRedisClient()
			if err := mgr.redisClient.Ping(); err != nil {
				log.Printf("auth-state: redis ping failed at startup (mode=%s): %v", mgr.mode, err)
			} else {
				log.Printf("auth-state: redis ping succeeded (mode=%s, db=%d)", mgr.mode, config.Data.Redis.DB)
			}
		}
		mgr.writePostgresFn = mgr.writePostgres
		mgr.validatePostgresFn = mgr.validatePostgres
		mgr.revokePostgresFn = mgr.revokePostgres
		globalAuthStateMgr = mgr
	})

	return globalAuthStateMgr
}

func (m *defaultAuthStateManager) Metrics() authStateMetricsSnapshot {
	return m.metrics.snapshot()
}

func (m *defaultAuthStateManager) sessionKey(username string) string {
	return fmt.Sprintf("%s:session:%s", m.keyPrefix, strings.ToLower(strings.TrimSpace(username)))
}

func secretHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func (m *defaultAuthStateManager) Issue(user *dbmodel.User, secret string) error {
	switch m.mode {
	case config.AuthStateModeDualWrite:
		if err := m.writePostgresFn(user, secret); err != nil {
			return err
		}
		if m.redisEnabled {
			if err := m.writeRedis(user.Username, secret); err != nil {
				return &AuthStateUnavailableError{Cause: err}
			}
		}
		return nil
	case config.AuthStateModeRedisPrimary:
		if err := m.writeRedis(user.Username, secret); err != nil {
			return &AuthStateUnavailableError{Cause: err}
		}
		if err := m.writePostgresFn(user, secret); err != nil {
			return err
		}
		return nil
	case config.AuthStateModePostgresOff:
		if err := m.writeRedis(user.Username, secret); err != nil {
			return &AuthStateUnavailableError{Cause: err}
		}
		if user.LoginToken != "" {
			user.LoginToken = ""
			if err := user.Update(database.Connection); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported auth state mode %q", m.mode)
	}
}

func (m *defaultAuthStateManager) Validate(username string, secret string) (bool, error) {
	start := time.Now()
	fallbackUsed := false

	defer func() {
		latencyMs := uint64(time.Since(start).Milliseconds())
		m.metrics.validateCount.Add(1)
		m.metrics.validateLatencyTotalMS.Add(latencyMs)
		if fallbackUsed {
			m.metrics.validateFallbackCount.Add(1)
		}
	}()

	switch m.mode {
	case config.AuthStateModeDualWrite:
		valid, err := m.validatePostgresFn(username, secret)
		if err != nil {
			m.metrics.validateErrorCount.Add(1)
			return false, err
		}
		if valid && m.redisEnabled {
			if redisResult, redisErr := m.validateRedis(username, secret); redisErr == nil && redisResult == authStateLookupMiss {
				if backfillErr := m.writeRedis(username, secret); backfillErr != nil {
					log.Printf("auth-state: redis backfill failed in dual-write mode username=%s: %v", username, backfillErr)
				}
			}
		}
		return valid, nil
	case config.AuthStateModeRedisPrimary:
		redisResult, err := m.validateRedis(username, secret)
		if err != nil {
			fallbackUsed = true
			valid, pgErr := m.validatePostgresFn(username, secret)
			if pgErr != nil {
				m.metrics.validateErrorCount.Add(1)
				return false, pgErr
			}
			if valid {
				if backfillErr := m.writeRedis(username, secret); backfillErr != nil {
					log.Printf("auth-state: redis backfill failed after redis error username=%s: %v", username, backfillErr)
				}
			}
			return valid, nil
		}
		if redisResult == authStateLookupValid {
			return true, nil
		}
		if redisResult == authStateLookupInvalid {
			return false, nil
		}

		fallbackUsed = true
		valid, pgErr := m.validatePostgresFn(username, secret)
		if pgErr != nil {
			m.metrics.validateErrorCount.Add(1)
			return false, pgErr
		}
		if valid {
			if backfillErr := m.writeRedis(username, secret); backfillErr != nil {
				log.Printf("auth-state: redis backfill failed after miss username=%s: %v", username, backfillErr)
			}
		}
		return valid, nil
	case config.AuthStateModePostgresOff:
		redisResult, err := m.validateRedis(username, secret)
		if err != nil {
			m.metrics.validateErrorCount.Add(1)
			return false, &AuthStateUnavailableError{Cause: err}
		}
		if redisResult == authStateLookupValid {
			return true, nil
		}
		return false, nil
	default:
		m.metrics.validateErrorCount.Add(1)
		return false, fmt.Errorf("unsupported auth state mode %q", m.mode)
	}
}

func (m *defaultAuthStateManager) Revoke(user *dbmodel.User) error {
	switch m.mode {
	case config.AuthStateModeDualWrite:
		if m.redisEnabled {
			if err := m.revokeRedis(user.Username); err != nil {
				m.metrics.revocationErrorCount.Add(1)
				return &AuthStateUnavailableError{Cause: err}
			}
		}
		return m.revokePostgresFn(user)
	case config.AuthStateModeRedisPrimary:
		if err := m.revokeRedis(user.Username); err != nil {
			m.metrics.revocationErrorCount.Add(1)
			return &AuthStateUnavailableError{Cause: err}
		}
		return m.revokePostgresFn(user)
	case config.AuthStateModePostgresOff:
		if err := m.revokeRedis(user.Username); err != nil {
			m.metrics.revocationErrorCount.Add(1)
			return &AuthStateUnavailableError{Cause: err}
		}
		return nil
	default:
		return fmt.Errorf("unsupported auth state mode %q", m.mode)
	}
}

func (m *defaultAuthStateManager) writeRedis(username string, secret string) error {
	if !m.redisEnabled || m.redisClient == nil {
		return errors.New("redis is not enabled")
	}

	record := authStateRecord{
		SecretHash: secretHash(secret),
		UpdatedAt:  time.Now().Unix(),
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return m.redisClient.SetEX(m.sessionKey(username), string(payload), m.sessionTTL)
}

func (m *defaultAuthStateManager) validateRedis(username string, secret string) (authStateLookup, error) {
	if !m.redisEnabled || m.redisClient == nil {
		return authStateLookupMiss, errors.New("redis is not enabled")
	}

	payload, exists, err := m.redisClient.Get(m.sessionKey(username))
	if err != nil {
		return authStateLookupMiss, err
	}
	if !exists {
		return authStateLookupMiss, nil
	}

	var record authStateRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return authStateLookupMiss, err
	}

	if record.SecretHash == secretHash(secret) {
		return authStateLookupValid, nil
	}
	return authStateLookupInvalid, nil
}

func (m *defaultAuthStateManager) revokeRedis(username string) error {
	if !m.redisEnabled || m.redisClient == nil {
		return errors.New("redis is not enabled")
	}
	return m.redisClient.Del(m.sessionKey(username))
}

func (m *defaultAuthStateManager) writePostgres(user *dbmodel.User, secret string) error {
	user.LoginToken = secret
	return user.Update(database.Connection)
}

func (m *defaultAuthStateManager) validatePostgres(username string, secret string) (bool, error) {
	var user dbmodel.User
	user.Username = username
	if err := user.GetUserByUsername(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if user.LoginToken == "" {
		return false, nil
	}
	if user.LoginToken != secret {
		return false, nil
	}
	return true, nil
}

func (m *defaultAuthStateManager) revokePostgres(user *dbmodel.User) error {
	user.LoginToken = ""
	return user.Update(database.Connection)
}
