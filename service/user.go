package service

import (
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"
	"time"
)

func CreateUser(input model.NewUser) (*dbmodel.User, error) {
	var user dbmodel.User
	user.Username = input.Username
	user.Role = input.Role
	// predefined value for a user
	user.LoginToken = ""
	user.IsActivated = true
	user.CreatedAt = time.Now()

	password, err := utils.GenerateHashedPassword(input.Password)
	if err != nil {
		return nil, err
	}
	user.Password = password

	if err := user.Create(); err != nil {
		return nil, err
	}

	return &dbmodel.User{
		Username: user.Username,
	}, nil
}

func GetAllUser(filter *model.UserFilter) ([]dbmodel.User, error) {
	var users []dbmodel.User
	query := database.Connection
	if filter != nil {
		if filter.Username != nil {
			query = query.Where("username = ?", filter.Username)
		}
		if filter.Role != nil {
			query = query.Where("role = ?", filter.Role)
		}
	}
	if err := query.Find(&users).Error; err != nil {
		return users, err
	}
	return users, nil
}
