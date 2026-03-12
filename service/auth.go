package service

import (
	"errors"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
	"strings"

	_ "github.com/shaj13/libcache/fifo"
)

type UnauthorizationError struct{}

func (m *UnauthorizationError) Error() string {
	return "wrong username or password"
}

type OIDCLoginRequiredError struct{}

func (m *OIDCLoginRequiredError) Error() string {
	return "oidc login is enabled; use /auth/oidc/start"
}

type TokenValidationError struct {
	Reason string
}

func (e *TokenValidationError) Error() string {
	if strings.TrimSpace(e.Reason) == "" {
		return "invalid token"
	}
	return "invalid token: " + strings.TrimSpace(e.Reason)
}

func Login(username string, password string) (string, error) {
	mode, err := config.ResolveAuthMode()
	if err != nil {
		log.Printf("cannot resolve auth mode for login: %v", err)
		return "", err
	}

	if mode == config.AuthModeOIDC {
		return "", &OIDCLoginRequiredError{}
	}

	return normalLogin(username, password)
}

func Logout(user *dbmodel.User) error {
	return getAuthStateManager().Revoke(user)
}

func ldapLogin(username string, password string) (string, error) {
	if err := LDAP(username, password); err != nil {
		return "", err
	}

	var user dbmodel.User
	user.Username = username
	exist := user.CheckUsernameExist(database.Connection)
	if !exist {
		user.Password = ""
		user.Role = config.Data.LDAP.DefaultRole
		user.IsActivated = true
		newtoken := utils.GenerateLoginKey()
		user.LoginToken = newtoken
		if err := user.Create(database.Connection); err != nil {
			return "", err
		}
	}

	if err := user.GetUserByUsername(database.Connection); err != nil {
		return "", err
	}

	return issueTokenForUser(&user)
}

func normalLogin(username string, password string) (string, error) {
	var user dbmodel.User
	user.Username = username

	if err := user.GetUserByUsername(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return "", &UnauthorizationError{}
		}
		return "", err
	}

	if !utils.CompareHash(user.Password, password) {
		return "", &UnauthorizationError{}
	}

	return issueTokenForUser(&user)
}

func ValidateToken(token string) (*dbmodel.User, error) {
	token = normalizeAuthToken(token)
	if token == "" {
		return nil, nil
	}

	jwtInfo, err := utils.ParseToken(token)
	if err != nil {
		log.Printf("token validation rejected: jwt parse failed: %v", err)
		return nil, &TokenValidationError{Reason: "jwt_parse_failed"}
	}

	valid, err := getAuthStateManager().Validate(jwtInfo.Username, jwtInfo.Secret)
	if err != nil {
		var unavailable *AuthStateUnavailableError
		if errors.As(err, &unavailable) {
			log.Printf("token validation unavailable for username=%s: %v", jwtInfo.Username, unavailable)
			return nil, unavailable
		}
		log.Printf("token validation failed for username=%s: %v", jwtInfo.Username, err)
		return nil, &TokenValidationError{Reason: "auth_state_error"}
	}
	if !valid {
		log.Printf("token validation rejected for username=%s: auth-state mismatch", jwtInfo.Username)
		return nil, &TokenValidationError{Reason: "auth_state_mismatch"}
	}

	var user dbmodel.User
	user.Username = jwtInfo.Username
	if err := user.GetUserByUsername(database.Connection); err != nil {
		log.Printf("token validation rejected for username=%s: user lookup failed: %v", jwtInfo.Username, err)
		if utils.RecordNotFound(err) {
			return nil, &TokenValidationError{Reason: "user_not_found"}
		}
		return nil, &TokenValidationError{Reason: "user_lookup_failed"}
	}

	return &user, nil
}

func normalizeAuthToken(token string) string {
	normalized := strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(normalized), "bearer ") {
		normalized = strings.TrimSpace(normalized[7:])
	}
	return normalized
}

func upsertUserAndGenerateToken(username string, defaultRole string) (string, error) {
	var user dbmodel.User
	user.Username = username
	exist := user.CheckUsernameExist(database.Connection)
	if !exist {
		user.Password = ""
		user.Role = defaultRole
		user.IsActivated = true
		newtoken := utils.GenerateLoginKey()
		user.LoginToken = newtoken
		if err := user.Create(database.Connection); err != nil {
			return "", err
		}
	}

	if err := user.GetUserByUsername(database.Connection); err != nil {
		return "", err
	}

	if user.Role == "" {
		user.Role = defaultRole
	}
	user.IsActivated = true
	if err := user.Update(database.Connection); err != nil {
		return "", err
	}

	return issueTokenForUser(&user)
}

func issueTokenForUser(user *dbmodel.User) (string, error) {
	sessionSecret := strings.TrimSpace(user.LoginToken)
	if sessionSecret == "" {
		sessionSecret = utils.GenerateLoginKey()
	}
	user.LoginToken = sessionSecret

	if err := getAuthStateManager().Issue(user, sessionSecret); err != nil {
		return "", err
	}

	jwtToken, err := utils.GenerateToken(user.Username, sessionSecret)
	if err != nil {
		return "", err
	}

	return jwtToken, nil
}
