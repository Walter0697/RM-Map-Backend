package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type TravelPlan struct {
	ObjectBase
	Relation    UserRelation          `gorm:"foreignKey:relation_id;references:id"`
	RelationID  uint                  `gorm:"index"`
	User        User                  `gorm:"foreignKey:user_id;references:id"`
	UserID      uint                  `gorm:"index"`
	Title       string                `gorm:"type:varchar(255);not null"`
	Description string                `gorm:"type:text"`
	StartDate   *time.Time            `gorm:"type:date"`
	EndDate     *time.Time            `gorm:"type:date"`
	Status      string                `gorm:"type:varchar(32);default:'draft';index"`
	DailyPlans  []TravelPlanDailyPlan `gorm:"foreignKey:travel_plan_id;constraint:OnDelete:CASCADE"`
}

type TravelPlanDailyPlan struct {
	BaseModel
	TravelPlan   TravelPlan `gorm:"foreignKey:travel_plan_id;references:id"`
	TravelPlanID uint       `gorm:"index"`
	Schedule     *Schedule  `gorm:"foreignKey:schedule_id;references:id"`
	ScheduleID   *uint      `gorm:"index"`
	DayIndex     int        `gorm:"index"`
	LocalDate    *time.Time `gorm:"type:date"`
	Summary      string     `gorm:"type:text;not null"`
	Details      string     `gorm:"type:text"`
}

func (plan *TravelPlan) GetWithDailyPlans(db *gorm.DB) error {
	return db.
		Preload("DailyPlans", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("day_index asc")
		}).
		Preload("Relation").
		Preload("User").
		Where("id = ?", plan.ID).
		First(plan).
		Error
}

func (plan *TravelPlan) Create(db *gorm.DB) error {
	return db.Create(plan).Error
}

func (plan *TravelPlan) Update(db *gorm.DB) error {
	return db.Save(plan).Error
}

func (daily *TravelPlanDailyPlan) DeleteByTravelPlanID(db *gorm.DB, travelPlanID uint) error {
	return db.Where("travel_plan_id = ?", travelPlanID).Delete(&TravelPlanDailyPlan{}).Error
}
