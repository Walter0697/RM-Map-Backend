package model

type PagedMarkerQuery struct {
	Cursor *string `json:"cursor"`
	Limit  *int    `json:"limit"`
}

