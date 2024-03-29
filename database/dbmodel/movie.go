package dbmodel

import "gorm.io/gorm"

type Movie struct {
	ObjectBase
	RefId       int          `json:"ref_id"`
	Label       string       `json:"label"`
	ReleaseDate *string      `json:"release_date"`
	ImageLink   string       `json:"imageLink"`
	IsFav       bool         `json:"is_fav"`
	Relation    UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId  uint
}

func (movie *Movie) Create(db *gorm.DB) error {
	if err := db.Create(movie).Error; err != nil {
		return err
	}

	return nil
}

func (movie *Movie) Update(db *gorm.DB) error {
	if err := db.Save(movie).Error; err != nil {
		return err
	}

	return nil
}

func (movie *Movie) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", movie.ID).First(movie).Error; err != nil {
		return err
	}

	return nil
}

func (movie *Movie) GetByRid(db *gorm.DB) error {
	if err := db.Where("ref_id = ?", movie.RefId).Where("relation_id", movie.RelationId).First(movie).Error; err != nil {
		return err
	}
	return nil
}
