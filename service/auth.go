package service

import (
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"

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
	user.LoginToken = ""

	if err := user.Update(database.Connection); err != nil {
		return err
	}

	return nil
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

func ValidateToken(token string) *dbmodel.User {
	jwtInfo, err := utils.ParseToken(token)
	if err != nil {
		return nil
	}

	var user dbmodel.User
	user.Username = jwtInfo.Username
	if err := user.GetUserByUsername(database.Connection); err != nil {
		return nil
	}

	if user.LoginToken == "" {
		return nil
	}

	if user.LoginToken != jwtInfo.Secret {
		return nil
	}

	return &user
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
	if user.LoginToken == "" {
		user.LoginToken = utils.GenerateLoginKey()
		if err := user.Update(database.Connection); err != nil {
			return "", err
		}
	}

	jwtToken, err := utils.GenerateToken(user.Username, user.LoginToken)
	if err != nil {
		return "", err
	}

	return jwtToken, nil
}
