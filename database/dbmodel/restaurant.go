package dbmodel

import "gorm.io/gorm"

type Restaurant struct {
	ObjectBase
	Name           string `json:"name"`
	Source         string `json:"source"`
	SourceId       string `json:"sourceId"`
	PhotoURL       string `json:"photoURL"`
	PriceRange     string `json:"priceRange"`
	RestaurantType string `json:"restaurantType"`
	Address        string `json:"address"`
	Rating         string `json:"rating"`
	Direction      string `json:"direction"`
	Telephone      string `json:"telephone"`
	Introduction   string `json:"introduction"`
	OpeningHours   string `json:"openingHours"`
	PaymentMethod  string `json:"paymentMethod"`
	SeatNo         string `json:"seatNo"`
	Website        string `json:"website"`
	OtherInfo      string `json:"otherInfo"`
}

func (restaurant *Restaurant) Create(db *gorm.DB) error {
	if err := db.Create(restaurant).Error; err != nil {
		return err
	}

	return nil
}

func (restaurant *Restaurant) Update(db *gorm.DB) error {
	if err := db.Save(restaurant).Error; err != nil {
		return err
	}
	return nil
}

func (restaurant *Restaurant) GetById(db *gorm.DB) error {
	if err := db.Where("id = ?", restaurant.ID).First(restaurant).Error; err != nil {
		return err
	}

	return nil
}

func (restaurant *Restaurant) GetBySourceId(db *gorm.DB) error {
	if err := db.Where("source_id = ?", restaurant.SourceId).First(restaurant).Error; err != nil {
		return err
	}

	return nil
}

func (restaurant *Restaurant) GetBySourceIdAndSource(db *gorm.DB) error {
	if err := db.Where("source_id = ? AND source = ?", restaurant.SourceId, restaurant.Source).First(restaurant).Error; err != nil {
		return err
	}

	return nil
}

func (restaurant *Restaurant) RemoveById(db *gorm.DB) error {
	if err := db.Where("id = ?", restaurant.ID).Delete(restaurant).Error; err != nil {
		return err
	}

	return nil
}
