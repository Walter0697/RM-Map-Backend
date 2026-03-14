package dbmodel

import "gorm.io/gorm"

type ServiceAccount struct {
	ObjectBase
	Name          string `json:"name" gorm:"uniqueIndex;not null"`
	Description   string `json:"description"`
	Role          string `json:"role" gorm:"index;not null"`
	Active        bool   `json:"active" gorm:"not null;default:true;index"`
	Relation      UserRelation `gorm:"foreignKey:relation_id;references:id"`
	RelationID    uint         `json:"relation_id" gorm:"not null;index"`
	ActingUser    *User  `gorm:"foreignKey:acting_user_id;references:id"`
	ActingUserID  *uint
}

func (serviceAccount *ServiceAccount) Create(db *gorm.DB) error {
	return db.Create(serviceAccount).Error
}

func (serviceAccount *ServiceAccount) Update(db *gorm.DB) error {
	return db.Save(serviceAccount).Error
}

func (serviceAccount *ServiceAccount) GetByID(db *gorm.DB) error {
	return db.Where("id = ?", serviceAccount.ID).Preload("ActingUser").Preload("Relation").First(serviceAccount).Error
}
