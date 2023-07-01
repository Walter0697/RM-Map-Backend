package dbmodel

import "gorm.io/gorm"

type CountryPoint struct {
	ObjectBase
	Label      string       `json:"label"`
	PhotoX     float64      `json:"photoX"`
	PhotoY     float64      `json:"photoY"`
	MapX       *float64     `json:"mapX"`
	MapY       *float64     `json:"mapY"`
	MapName    string       `json:"mapName"`
	Relation   UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId uint
}

func (point *CountryPoint) Create(db *gorm.DB) error {
	if err := db.Create(point).Error; err != nil {
		return err
	}

	return nil
}

func (point *CountryPoint) Update(db *gorm.DB) error {
	if err := db.Save(point).Error; err != nil {
		return err
	}

	return nil
}

func (point *CountryPoint) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", point.ID).First(point).Error; err != nil {
		return err
	}

	return nil
}
