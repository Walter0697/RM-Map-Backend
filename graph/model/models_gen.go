// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"github.com/99designs/gqlgen/graphql"
)

type CountryCodeMap struct {
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name"`
}

type CurrentTime struct {
	Time string `json:"time"`
}

type DefaultPin struct {
	Label     string  `json:"label"`
	Pin       *Pin    `json:"pin"`
	CreatedAt *string `json:"created_at"`
	CreatedBy *User   `json:"created_by"`
	UpdatedAt *string `json:"updated_at"`
	UpdatedBy *User   `json:"updated_by"`
}

type EventType struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Priority int    `json:"priority"`
	IconPath string `json:"icon_path"`
	Hidden   bool   `json:"hidden"`
}

type IDModel struct {
	ID int `json:"id"`
}

type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Jwt      string `json:"jwt"`
	Username string `json:"username"`
}

type Logout struct {
	Jwt string `json:"jwt"`
}

type ManageRoroadList struct {
	Ids    []*int `json:"ids"`
	Hidden *bool  `json:"hidden"`
}

type MapPin struct {
	Pinlabel  string `json:"pinlabel"`
	Typelabel string `json:"typelabel"`
	ImagePath string `json:"image_path"`
}

type Marker struct {
	ID           int         `json:"id"`
	Label        string      `json:"label"`
	Latitude     float64     `json:"latitude"`
	Longitude    float64     `json:"longitude"`
	Address      string      `json:"address"`
	ImageLink    *string     `json:"image_link"`
	Link         *string     `json:"link"`
	Type         string      `json:"type"`
	Description  *string     `json:"description"`
	EstimateTime *string     `json:"estimate_time"`
	Price        *string     `json:"price"`
	Permanent    bool        `json:"permanent"`
	NeedBooking  bool        `json:"need_booking"`
	Status       *string     `json:"status"`
	ToTime       *string     `json:"to_time"`
	FromTime     *string     `json:"from_time"`
	Restaurant   *Restaurant `json:"restaurant"`
	IsFav        bool        `json:"is_fav"`
	CountryCode  string      `json:"country_code"`
	CountryPart  string      `json:"country_part"`
	CreatedAt    string      `json:"created_at"`
	CreatedBy    *User       `json:"created_by"`
	UpdatedAt    string      `json:"updated_at"`
	UpdatedBy    *User       `json:"updated_by"`
}

type MarkerType struct {
	ID        int    `json:"id"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	Priority  int    `json:"priority"`
	Hidden    bool   `json:"hidden"`
	IconPath  string `json:"icon_path"`
	CreatedAt string `json:"created_at"`
	CreatedBy *User  `json:"created_by"`
	UpdatedAt string `json:"updated_at"`
	UpdatedBy *User  `json:"updated_by"`
}

type MetaDataOutput struct {
	ImageLink string `json:"image_link"`
	Title     string `json:"title"`
}

type Movie struct {
	ID          int     `json:"id"`
	ReferenceID int     `json:"reference_id"`
	Label       string  `json:"label"`
	ReleaseDate *string `json:"release_date"`
	ImagePath   *string `json:"image_path"`
	IsFav       bool    `json:"is_fav"`
	CreatedAt   string  `json:"created_at"`
	CreatedBy   *User   `json:"created_by"`
	UpdatedAt   string  `json:"updated_at"`
	UpdatedBy   *User   `json:"updated_by"`
}

type MovieFilter struct {
	Type     string  `json:"type"`
	Location *string `json:"location"`
	Query    *string `json:"query"`
}

type MovieOutput struct {
	RefID       int    `json:"ref_id"`
	Title       string `json:"title"`
	ImageLink   string `json:"image_link"`
	ReleaseDate string `json:"release_date"`
}

type NewFavouriteMovie struct {
	MovieRid int `json:"movie_rid"`
}

type NewMarker struct {
	Label        string          `json:"label"`
	Latitude     float64         `json:"latitude"`
	Longitude    float64         `json:"longitude"`
	Address      string          `json:"address"`
	ImageLink    *string         `json:"image_link"`
	ImageUpload  *graphql.Upload `json:"image_upload"`
	Link         *string         `json:"link"`
	Type         string          `json:"type"`
	Description  *string         `json:"description"`
	Permanent    *bool           `json:"permanent"`
	NeedBooking  *bool           `json:"need_booking"`
	ToTime       *string         `json:"to_time"`
	FromTime     *string         `json:"from_time"`
	EstimateTime *string         `json:"estimate_time"`
	RestaurantID *int            `json:"restaurant_id"`
	Price        *string         `json:"price"`
}

type NewMarkerType struct {
	Label      string          `json:"label"`
	Value      string          `json:"value"`
	Priority   int             `json:"priority"`
	IconUpload *graphql.Upload `json:"icon_upload"`
	Hidden     bool            `json:"hidden"`
}

type NewMovieSchedule struct {
	Label        string `json:"label"`
	Description  string `json:"description"`
	SelectedTime string `json:"selected_time"`
	MovieRid     int    `json:"movie_rid"`
	MarkerID     *int   `json:"marker_id"`
}

type NewPin struct {
	Label        string          `json:"label"`
	TopLeftX     int             `json:"top_left_x"`
	TopLeftY     int             `json:"top_left_y"`
	BottomRightX int             `json:"bottom_right_x"`
	BottomRightY int             `json:"bottom_right_y"`
	ImageUpload  *graphql.Upload `json:"image_upload"`
}

type NewRoroadList struct {
	Name       string `json:"name"`
	TargetUser string `json:"target_user"`
	ListType   string `json:"list_type"`
}

type NewSchedule struct {
	Label        string `json:"label"`
	Description  string `json:"description"`
	SelectedTime string `json:"selected_time"`
	MarkerID     int    `json:"marker_id"`
}

type NewUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type Pin struct {
	ID           int    `json:"id"`
	Label        string `json:"label"`
	ImagePath    string `json:"image_path"`
	DisplayPath  string `json:"display_path"`
	TopLeftX     int    `json:"top_left_x"`
	TopLeftY     int    `json:"top_left_y"`
	BottomRightX int    `json:"bottom_right_x"`
	BottomRightY int    `json:"bottom_right_y"`
	CreatedAt    string `json:"created_at"`
	CreatedBy    *User  `json:"created_by"`
	UpdatedAt    string `json:"updated_at"`
	UpdatedBy    *User  `json:"updated_by"`
}

type PreviewPinInput struct {
	TopLeftX     int             `json:"top_left_x"`
	TopLeftY     int             `json:"top_left_y"`
	BottomRightX int             `json:"bottom_right_x"`
	BottomRightY int             `json:"bottom_right_y"`
	ImageUpload  *graphql.Upload `json:"image_upload"`
	TypeID       int             `json:"type_id"`
}

type ReleaseNote struct {
	Version string  `json:"version"`
	Notes   *string `json:"notes"`
	Date    *string `json:"date"`
	Icon    *string `json:"icon"`
}

type ReleaseNoteFilter struct {
	Version string `json:"version"`
}

type RemoveModel struct {
	ID int `json:"id"`
}

type Restaurant struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Source         string  `json:"source"`
	SourceID       string  `json:"source_id"`
	PriceRange     *string `json:"price_range"`
	RestaurantType *string `json:"restaurant_type"`
	Address        *string `json:"address"`
	Rating         *string `json:"rating"`
	Direction      *string `json:"direction"`
	Telephone      *string `json:"telephone"`
	Introduction   *string `json:"introduction"`
	OpeningHours   *string `json:"opening_hours"`
	PaymentMethod  *string `json:"payment_method"`
	SeatNumber     *string `json:"seat_number"`
	Website        *string `json:"website"`
	OtherInfo      *string `json:"other_info"`
}

type RoroadList struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ListType   string `json:"list_type"`
	Checked    bool   `json:"checked"`
	Hidden     bool   `json:"hidden"`
	TargetUser string `json:"target_user"`
}

type RoroadListSearchFilter struct {
	Name   *string `json:"name"`
	Hidden *bool   `json:"hidden"`
}

type Schedule struct {
	ID           int     `json:"id"`
	Label        string  `json:"label"`
	Description  string  `json:"description"`
	Status       string  `json:"status"`
	SelectedDate string  `json:"selected_date"`
	Marker       *Marker `json:"marker"`
	Movie        *Movie  `json:"movie"`
	CreatedAt    string  `json:"created_at"`
	CreatedBy    *User   `json:"created_by"`
	UpdatedAt    string  `json:"updated_at"`
	UpdatedBy    *User   `json:"updated_by"`
}

type ScheduleStatus struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type ScheduleStatusList struct {
	Ids []*ScheduleStatus `json:"ids"`
}

type Station struct {
	Identifier string  `json:"identifier"`
	Label      string  `json:"label"`
	LocalName  string  `json:"local_name"`
	PhotoX     float64 `json:"photo_x"`
	PhotoY     float64 `json:"photo_y"`
	MapX       float64 `json:"map_x"`
	MapY       float64 `json:"map_y"`
	Active     bool    `json:"active"`
	MapName    string  `json:"map_name"`
	LineInfo   string  `json:"line_info"`
}

type TodayEvent struct {
	YesterdayEvent []*Schedule `json:"yesterday_event"`
}

type UpdateMarker struct {
	ID               int             `json:"id"`
	Label            *string         `json:"label"`
	Address          *string         `json:"address"`
	ImageLink        *string         `json:"image_link"`
	ImageUpload      *graphql.Upload `json:"image_upload"`
	NoImage          bool            `json:"no_image"`
	Link             *string         `json:"link"`
	Type             *string         `json:"type"`
	Description      *string         `json:"description"`
	Permanent        *bool           `json:"permanent"`
	NeedBooking      *bool           `json:"need_booking"`
	ToTime           *string         `json:"to_time"`
	FromTime         *string         `json:"from_time"`
	EstimateTime     *string         `json:"estimate_time"`
	RestaurantID     *int            `json:"restaurant_id"`
	RemoveRestaurant *bool           `json:"remove_restaurant"`
	Price            *string         `json:"price"`
}

type UpdateMarkerFavourite struct {
	ID    int  `json:"id"`
	IsFav bool `json:"is_fav"`
}

type UpdateModel struct {
	ID int `json:"id"`
}

type UpdatePreferredPin struct {
	Label string `json:"label"`
	PinID *int   `json:"pin_id"`
}

type UpdateRelation struct {
	Username string `json:"username"`
}

type UpdateRoroadList struct {
	ID         int     `json:"id"`
	Name       *string `json:"name"`
	ListType   *string `json:"list_type"`
	Checked    *bool   `json:"checked"`
	Hidden     *bool   `json:"hidden"`
	TargetUser *string `json:"target_user"`
}

type UpdateSchedule struct {
	ID           int     `json:"id"`
	Label        *string `json:"label"`
	Description  *string `json:"description"`
	SelectedTime *string `json:"selected_time"`
}

type UpdateStation struct {
	MapName    string `json:"map_name"`
	Identifier string `json:"identifier"`
	Active     bool   `json:"active"`
}

type UpdatedDefault struct {
	Label       string  `json:"label"`
	UpdatedType string  `json:"updated_type"`
	IntValue    *int    `json:"int_value"`
	StringValue *string `json:"string_value"`
}

type UpdatedMarkerType struct {
	ID         int             `json:"id"`
	Label      *string         `json:"label"`
	Value      *string         `json:"value"`
	Priority   *int            `json:"priority"`
	IconUpload *graphql.Upload `json:"icon_upload"`
	Hidden     *bool           `json:"hidden"`
}

type UpdatedPin struct {
	ID           int             `json:"id"`
	Label        *string         `json:"label"`
	TopLeftX     *int            `json:"top_left_x"`
	TopLeftY     *int            `json:"top_left_y"`
	BottomRightX *int            `json:"bottom_right_x"`
	BottomRightY *int            `json:"bottom_right_y"`
	ImageUpload  *graphql.Upload `json:"image_upload"`
}

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type UserFilter struct {
	Username *string `json:"username"`
	Role     *string `json:"role"`
}

type UserPreference struct {
	ID           int   `json:"id"`
	User         *User `json:"user"`
	Relation     *User `json:"relation"`
	RegularPin   *Pin  `json:"regular_pin"`
	FavouritePin *Pin  `json:"favourite_pin"`
	SelectedPin  *Pin  `json:"selected_pin"`
	HurryPin     *Pin  `json:"hurry_pin"`
}

type UserSearch struct {
	Username string `json:"username"`
}

type WebLink struct {
	Link string `json:"link"`
}

type WebsiteScrapInput struct {
	Source   string `json:"source"`
	SourceID string `json:"source_id"`
}

type WebsiteScrapResult struct {
	Restaurant *Restaurant `json:"restaurant"`
}
