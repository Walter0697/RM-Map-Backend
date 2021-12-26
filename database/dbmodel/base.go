package dbmodel

import (
	"log"
	"mapmarker/backend/database"
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uint           `json:"id" gorm:"primary_key"`
	CreatedAt time.Time      `json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type ObjectBase struct {
	BaseModel
	CreatedBy  *User `gorm:"foreignKey:created_uid;references:id"`
	CreatedUID *uint
	UpdatedAt  time.Time `json:"updatedAt"`
	UpdatedBy  *User     `gorm:"foreignKey:updated_uid;references:id"`
	UpdatedUID *uint
}

// note: we don't really need to put json tag in dbmodel but I did it anyway
func AutoMigration() {
	database.Connection.AutoMigrate(&ReleaseNote{})
	database.Connection.AutoMigrate(&User{})
	database.Connection.AutoMigrate(&UserRelation{})
	database.Connection.AutoMigrate(&UserPreference{})
	database.Connection.AutoMigrate(&Marker{})
	database.Connection.AutoMigrate(&MarkerType{})
	database.Connection.AutoMigrate(&Pin{})
	database.Connection.AutoMigrate(&TypePin{})
	database.Connection.AutoMigrate(&DefaultValue{})
	database.Connection.AutoMigrate(&Schedule{})
	database.Connection.AutoMigrate(&Movie{})

	log.Println("auto migration completed")
}
