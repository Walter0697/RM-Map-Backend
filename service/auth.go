package service

import (
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"

	_ "github.com/shaj13/libcache/fifo"
)

type UnauthorizationError struct{}

func (m *UnauthorizationError) Error() string {
	return "wrong username or password"
}

func Login(username string, password string) (string, error) {
	if config.Data.LDAP.Enable {
		return ldapLogin(username, password)
	}
	return normalLogin(username, password)
}

func ldapLogin(username string, password string) (string, error) {
	if err := LDAP(username, password); err != nil {
		return "", err
	}

	var user dbmodel.User
	user.Username = username
	exist := user.CheckUsernameExist()
	if !exist {
		user.Password = ""
		user.Role = config.Data.LDAP.DefaultRole
		user.IsActivated = true
		newtoken := utils.GenerateLoginKey()
		user.LoginToken = newtoken
		if err := user.Create(); err != nil {
			return "", err
		}
	}

	if err := user.GetUserByUsername(); err != nil {
		return "", err
	}

	if user.LoginToken == "" {
		newtoken := utils.GenerateLoginKey()
		user.LoginToken = newtoken
		if err := user.Update(); err != nil {
			return "", err
		}
	}

	jwtToken, err := utils.GenerateToken(user.Username, user.LoginToken)
	if err != nil {
		return "", err
	}

	return jwtToken, nil
}

func normalLogin(username string, password string) (string, error) {
	var user dbmodel.User
	user.Username = username

	if err := user.GetUserByUsername(); err != nil {
		if utils.RecordNotFound(err) {
			return "", &UnauthorizationError{}
		}
		return "", err
	}

	if !utils.CompareHash(user.Password, password) {
		return "", &UnauthorizationError{}
	}

	// login successfully, now retrieve or generate a random key
	if user.LoginToken == "" {
		newtoken := utils.GenerateLoginKey()
		user.LoginToken = newtoken
		if err := user.Update(); err != nil {
			return "", err
		}
	}

	jwtToken, err := utils.GenerateToken(user.Username, user.LoginToken)
	if err != nil {
		return "", err
	}

	return jwtToken, nil
}

func ValidateToken(token string) *dbmodel.User {
	jwtInfo, err := utils.ParseToken(token)
	if err != nil {
		return nil
	}

	var user dbmodel.User
	user.Username = jwtInfo.Username
	if err := user.GetUserByUsername(); err != nil {
		return nil
	}

	if user.LoginToken != jwtInfo.Secret {
		return nil
	}

	return &user
}
