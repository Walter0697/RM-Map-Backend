package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

type Schedule struct {
	ObjectBase
	Label                        string    `json:"label"`
	Testing                      bool      `json:"testing" gorm:"not null;default:false;index"`
	Description                  string    `json:"description"`
	Status                       string    `json:"status"`
	SelectedDate                 time.Time `json:"selectedDate"`
	WeatherCondition             *string   `json:"weather_condition,omitempty" gorm:"type:varchar(64)"`
	WeatherForecastAt            *time.Time
	WeatherTemperature           *float64 `json:"weather_temperature,omitempty"`
	WeatherTemperatureUnit       *string  `json:"weather_temperature_unit,omitempty" gorm:"type:varchar(16)"`
	WeatherSource                *string  `json:"weather_source,omitempty" gorm:"type:varchar(64)"`
	WeatherFetchedAt             *time.Time
	WeatherStatus                *string `json:"weather_status,omitempty" gorm:"type:varchar(32)"`
	WeatherUnavailableReason     *string `json:"weather_unavailable_reason,omitempty" gorm:"type:varchar(64)"`
	RouteImageRef                *string `json:"route_image_ref,omitempty" gorm:"type:text"`
	RouteDistanceMeters          *int    `json:"route_distance_meters,omitempty"`
	RouteETAWalkingSeconds       *int    `json:"route_eta_walking_seconds,omitempty"`
	RouteETABusSeconds           *int    `json:"route_eta_bus_seconds,omitempty"`
	RouteETAPublicTransitSeconds *int    `json:"route_eta_public_transit_seconds,omitempty"`
	RoutePreviewWarning          *string `json:"route_preview_warning,omitempty" gorm:"type:text"`
	SelectedMarker               *Marker `gorm:"foreignKey:marker_id;reference:id"`
	MarkerId                     *uint
	SelectedMovie                *Movie `gorm:"foreignKey:movie_id;referenece:id"`
	MovieId                      *uint
	Relation                     UserRelation `gorm:"foreignKey:relation_id;reference:id"`
	RelationId                   uint
}

func (schedule *Schedule) Create(db *gorm.DB) error {
	if err := db.Create(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) Update(db *gorm.DB) error {
	if err := db.Save(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) GetById(db *gorm.DB) error {
	if err := db.Preload("SelectedMarker").Preload("SelectedMovie").Where("id = ?", schedule.ID).First(schedule).Error; err != nil {
		return err
	}

	return nil
}

func (schedule *Schedule) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", schedule.ID).Delete(schedule).Error; err != nil {
		return err
	}

	return nil
}
