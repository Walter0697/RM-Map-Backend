package dbmodel

import "gorm.io/gorm"

type UserPreference struct {
	BaseModel
	CurrentUser      User `gorm:"foreignKey:user_id;reference:id"`
	UserId           uint
	SelectedRelation *UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId       *uint
	RegularPin       *Pin `gorm:"foreignKey:rpin_id;reference:id"` // selected pin
	RpinId           *uint
	FavouritePin     *Pin `gorm:"foreignKey:fpin_id;reference:id"`
	FpinId           *uint
	SelectedPin      *Pin `gorm:"foreignKey:spin_id;reference:id"`
	SpinId           *uint
	HurryPin         *Pin `gorm:"foreignKey:hpin_id;reference:id"`
	HpinId           *uint
}

func (preference *UserPreference) Create(db *gorm.DB) error {
	if err := db.Create(preference).Error; err != nil {
		return err
	}

	return nil
}

func (preference *UserPreference) Update(db *gorm.DB) error {
	if err := db.Save(preference).Error; err != nil {
		return err
	}

	return nil
}

func (preference *UserPreference) GetOrCreateByUserId(db *gorm.DB) error {
	if err := db.Where("user_id = ?", preference.CurrentUser.ID).FirstOrCreate(preference).Error; err != nil {
		return err
	}

	return nil
}

func (preference *UserPreference) GetByUserId(db *gorm.DB) error {
	if err := db.
		Preload("RegularPin").
		Preload("FavouritePin").
		Preload("SelectedPin").
		Preload("HurryPin").
		Where("user_id = ?", preference.UserId).
		First(preference).Error; err != nil {
		return err
	}

	return nil
}
