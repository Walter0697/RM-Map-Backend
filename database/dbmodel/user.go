package dbmodel

import (
	"mapmarker/backend/database"
)

type User struct {
	BaseModel
	Username    string `json:"username"`
	Password    string `json:"-"`
	LoginToken  string `json:"token"`
	Role        string `json:"role"`
	IsActivated bool   `json:"is_activated"`
}

func (user *User) Create() error {
	if err := database.Connection.Create(user).Error; err != nil {
		return err
	}

	return nil
}

func (user *User) Update() error {
	if err := database.Connection.Save(user).Error; err != nil {
		return err
	}

	return nil
}

func (user *User) GetUserByUsername() error {
	if err := database.Connection.Where("username = ?", user.Username).Find(user).Error; err != nil {
		return err
	}

	return nil
}

func (user *User) GetUserById() error {
	if err := database.Connection.Where("id = ?", user.ID).Find(user).Error; err != nil {
		return err
	}

	return nil
}

func (user User) CheckUsernameExist() bool {
	var count int64
	if err := database.Connection.Model(&user).Where("username = ?", user.Username).Count(&count).Error; err != nil {
		return false
	}

	return count != 0
}
