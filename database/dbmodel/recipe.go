package dbmodel

import "gorm.io/gorm"

type Recipe struct {
	ObjectBase
	Relation    UserRelation       `gorm:"foreignKey:relation_id;references:id"`
	RelationID  uint               `gorm:"index"`
	User        User               `gorm:"foreignKey:user_id;references:id"`
	UserID      uint               `gorm:"index"`
	Title       string             `gorm:"type:varchar(255);not null"`
	Description string             `gorm:"type:text"`
	Ingredients []RecipeIngredient `gorm:"foreignKey:recipe_id;constraint:OnDelete:CASCADE"`
	Steps       []RecipeStep       `gorm:"foreignKey:recipe_id;constraint:OnDelete:CASCADE"`
}

type RecipeIngredient struct {
	BaseModel
	Recipe    Recipe `gorm:"foreignKey:recipe_id;references:id"`
	RecipeID  uint   `gorm:"index"`
	SortOrder int    `gorm:"index"`
	Name      string `gorm:"type:varchar(255);not null"`
	Quantity  string `gorm:"type:varchar(255)"`
	Notes     string `gorm:"type:text"`
}

type RecipeStep struct {
	BaseModel
	Recipe      Recipe `gorm:"foreignKey:recipe_id;references:id"`
	RecipeID    uint   `gorm:"index"`
	SortOrder   int    `gorm:"index"`
	Instruction string `gorm:"type:text;not null"`
}

func (recipe *Recipe) GetWithDetails(db *gorm.DB) error {
	return db.
		Preload("Ingredients", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("sort_order asc").Order("id asc")
		}).
		Preload("Steps", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("sort_order asc").Order("id asc")
		}).
		Preload("Relation").
		Preload("User").
		Where("id = ?", recipe.ID).
		First(recipe).
		Error
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
