package dbmodel

import "gorm.io/gorm"

type RoRoadList struct {
	ObjectBase
	Name       string       `json:"name"`
	Checked    bool         `json:"checked"`
	Hidden     bool         `json:"hidden"`
	Type       string       `json:"type"`
	TargetUser string       `json:"target_user"`
	Relation   UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId uint
}

func (roroadlist *RoRoadList) Create(db *gorm.DB) error {
	if err := db.Create(roroadlist).Error; err != nil {
		return err
	}

	return nil
}

func (roroadlist *RoRoadList) Update(db *gorm.DB) error {
	if err := db.Save(roroadlist).Error; err != nil {
		return err
	}
	return nil
}

func (roroadlist *RoRoadList) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", roroadlist.ID).First(roroadlist).Error; err != nil {
		return err
	}

	return nil
}
