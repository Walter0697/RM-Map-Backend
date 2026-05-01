package dbmodel

import "gorm.io/gorm"

type Recipe struct {
	ObjectBase
	Relation     UserRelation `gorm:"foreignKey:relation_id;references:id"`
	RelationID   uint         `gorm:"index"`
	User         User         `gorm:"foreignKey:user_id;references:id"`
	UserID       uint         `gorm:"index"`
	Title        string       `gorm:"type:varchar(255);not null"`
	Description  string       `gorm:"type:text"`
	Ingredients  string       `gorm:"type:text"`
	Instructions string       `gorm:"type:text"`
	ServingSize  string       `gorm:"type:varchar(64)"`
	PrepMinutes  int          `gorm:"default:0"`
	CookMinutes  int          `gorm:"default:0"`
	Tags         string       `gorm:"type:text"`
}

func (recipe *Recipe) Create(db *gorm.DB) error {
	return db.Create(recipe).Error
}

func (recipe *Recipe) Update(db *gorm.DB) error {
	return db.Save(recipe).Error
}

func (recipe *Recipe) Delete(db *gorm.DB) error {
	return db.Delete(recipe).Error
}

func (recipe *Recipe) GetByID(db *gorm.DB) error {
	return db.Preload("Relation").Preload("User").Where("id = ?", recipe.ID).First(recipe).Error
}
