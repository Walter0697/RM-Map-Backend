// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"github.com/99designs/gqlgen/graphql"
)

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
}

type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Jwt      string `json:"jwt"`
	Username string `json:"username"`
}

type Marker struct {
	ID           int     `json:"id"`
	Label        string  `json:"label"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Address      string  `json:"address"`
	ImageLink    *string `json:"image_link"`
	Link         *string `json:"link"`
	Type         string  `json:"type"`
	Description  *string `json:"description"`
	EstimateTime *string `json:"estimate_time"`
	Price        *string `json:"price"`
	Status       *string `json:"status"`
	ToTime       *string `json:"to_time"`
	FromTime     *string `json:"from_time"`
	IsFav        bool    `json:"is_fav"`
	CreatedAt    string  `json:"created_at"`
	CreatedBy    *User   `json:"created_by"`
	UpdatedAt    string  `json:"updated_at"`
	UpdatedBy    *User   `json:"updated_by"`
}

type MarkerType struct {
	ID        int    `json:"id"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	Priority  int    `json:"priority"`
	IconPath  string `json:"icon_path"`
	CreatedAt string `json:"created_at"`
	CreatedBy *User  `json:"created_by"`
	UpdatedAt string `json:"updated_at"`
	UpdatedBy *User  `json:"updated_by"`
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
	ToTime       *string         `json:"to_time"`
	FromTime     *string         `json:"from_time"`
	EstimateTime *string         `json:"estimate_time"`
	Price        *string         `json:"price"`
}

type NewMarkerType struct {
	Label      string          `json:"label"`
	Value      string          `json:"value"`
	Priority   int             `json:"priority"`
	IconUpload *graphql.Upload `json:"icon_upload"`
}

type NewPin struct {
	Label        string          `json:"label"`
	TopLeftX     int             `json:"top_left_x"`
	TopLeftY     int             `json:"top_left_y"`
	BottomRightX int             `json:"bottom_right_x"`
	BottomRightY int             `json:"bottom_right_y"`
	ImageUpload  *graphql.Upload `json:"image_upload"`
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

type RemoveModel struct {
	ID int `json:"id"`
}

type UpdateMarkerFavourite struct {
	ID    int  `json:"id"`
	IsFav bool `json:"is_fav"`
}

type UpdatePreferredPin struct {
	Label string `json:"label"`
	PinID *int   `json:"pin_id"`
}

type UpdateRelation struct {
	Username string `json:"username"`
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
