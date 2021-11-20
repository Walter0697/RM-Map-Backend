package dbmodel

import (
	"gorm.io/gorm"
)

type User struct {
	BaseModel
	Username    string `json:"username"`
	Password    string `json:"-"`
	LoginToken  string `json:"token"`
	Role        string `json:"role"`
	IsActivated bool   `json:"is_activated"`
}

func (user *User) Create(db *gorm.DB) error {
	if err := db.Create(user).Error; err != nil {
		return err
	}

	return nil
}

func (user *User) Update(db *gorm.DB) error {
	if err := db.Save(user).Error; err != nil {
		return err
	}

	return nil
}

func (user *User) GetUserByUsername(db *gorm.DB) error {
	if err := db.Where("username = ?", user.Username).First(user).Error; err != nil {
		return err
	}

	return nil
}

func (user *User) GetUserById(db *gorm.DB) error {
	if err := db.Where("id = ?", user.ID).First(user).Error; err != nil {
		return err
	}

	return nil
}

func (user User) CheckUsernameExist(db *gorm.DB) bool {
	var count int64
	if err := db.Model(&user).Where("username = ?", user.Username).Count(&count).Error; err != nil {
		return false
	}

	return count != 0
}
