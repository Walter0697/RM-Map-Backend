package service

import (
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"
	"time"
)

func CreateMarker(input model.NewMarker, user dbmodel.User) (*dbmodel.Marker, error) {
	var fromTime time.Time
	var toTime time.Time
	var err error
	if input.FromTime != nil {
		fromTime, err = time.Parse(time.RFC3339, *input.FromTime)
		if err != nil {
			return nil, err
		}
	}
	if input.ToTime != nil {
		toTime, err = time.Parse(time.RFC3339, *input.ToTime)
		if err != nil {
			return nil, err
		}
	}

	var marker dbmodel.Marker

	marker.Label = input.Label
	marker.Latitude = input.Latitude
	marker.Longitude = input.Longitude
	marker.Address = input.Address
	if input.ImageLink != nil {
		marker.ImageLink = *input.ImageLink
	}
	if input.Link != nil {
		marker.Link = *input.Link
	}
	marker.Type = input.Type
	marker.Description = *input.Description
	if input.FromTime != nil {
		marker.FromTime = &fromTime
	}
	if input.ToTime != nil {
		marker.ToTime = &toTime
	}

	marker.CreatedBy = &user
	marker.UpdatedBy = &user

	if err := marker.Create(); err != nil {
		return nil, err
	}

	return &dbmodel.Marker{}, nil
}

func GetAllActiveMarker(requested []string) ([]dbmodel.Marker, error) {
	var markers []dbmodel.Marker
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	if err := query.Find(&markers).Error; err != nil {
		return markers, err
	}
	return markers, nil
}
