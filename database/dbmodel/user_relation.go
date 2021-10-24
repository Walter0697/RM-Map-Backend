package dbmodel

import (
	"mapmarker/backend/database"
)

type UserRelation struct {
	BaseModel
	UserOne    User `gorm:"foreignKey:user_one_uid;references:id"`
	UserOneUID uint
	UserTwo    User `gorm:"foreignKey:user_two_uid;references:id"`
	UserTwoUID uint
}

func (relation *UserRelation) Create() error {
	if err := database.Connection.Create(relation).Error; err != nil {
		return err
	}

	return nil
}

func (relation *UserRelation) GetOrCreateByUsers() error {
	if err := database.Connection.Where("(user_one_uid = ? AND user_two_uid = ?) OR (user_one_uid = ? AND user_two_uid = ?)", relation.UserOne.ID, relation.UserTwo.ID, relation.UserTwo.ID, relation.UserOne.ID).FirstOrCreate(relation).Error; err != nil {
		return err
	}

	return nil
}

func (relation *UserRelation) GetRelationByUsers() error {
	if err := database.Connection.Where("(user_one_uid = ? AND user_two_uid = ?) OR (user_one_uid = ? AND user_two_uid = ?)", relation.UserOneUID, relation.UserTwoUID, relation.UserTwoUID, relation.UserOneUID).First(relation).Error; err != nil {
		return err
	}

	return nil
}

func (relation *UserRelation) GetRelationById() error {
	if err := database.Connection.Where("id = ?", relation.ID).Preload("UserOne").Preload("UserTwo").First(relation).Error; err != nil {
		return err
	}

	return nil
}
