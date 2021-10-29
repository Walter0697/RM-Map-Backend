package helper

import (
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"
)

func ConvertUser(user dbmodel.User) model.User {
	var item model.User
	item.ID = int(user.ID)
	item.Username = user.Username
	item.Role = user.Role
	item.CreatedAt = utils.ConvertToOutputTime(user.CreatedAt)
	return item
}

func ConvertMarker(marker dbmodel.Marker) model.Marker {
	var item model.Marker
	item.ID = int(marker.ID)
	item.Label = marker.Label
	item.Latitude = marker.Latitude
	item.Longitude = marker.Longitude
	item.Address = marker.Address
	item.ImageLink = &marker.ImageLink
	item.Link = &marker.Link
	item.Type = marker.Type
	item.Description = &marker.Description
	item.EstimateTime = &marker.EstimateTime
	item.Price = &marker.Price
	item.Status = &marker.Status

	item.CreatedAt = utils.ConvertToOutputTime(marker.CreatedAt)
	item.UpdatedAt = utils.ConvertToOutputTime(marker.UpdatedAt)
	if marker.CreatedBy != nil {
		createdBy := ConvertUser(*marker.CreatedBy)
		item.CreatedBy = &createdBy
	}
	if marker.UpdatedBy != nil {
		updatedBy := ConvertUser(*marker.UpdatedBy)
		item.UpdatedBy = &updatedBy
	}

	item.IsFav = marker.IsFavourite
	return item
}

func ConvertMarkerType(markertype dbmodel.MarkerType) model.MarkerType {
	var item model.MarkerType
	item.ID = int(markertype.ID)
	item.Label = markertype.Label
	item.Value = markertype.Value
	item.Priority = markertype.Priority
	item.IconPath = markertype.IconPath

	item.CreatedAt = utils.ConvertToOutputTime(markertype.CreatedAt)
	item.UpdatedAt = utils.ConvertToOutputTime(markertype.UpdatedAt)
	if markertype.CreatedBy != nil {
		createdBy := ConvertUser(*markertype.CreatedBy)
		item.CreatedBy = &createdBy
	}
	if markertype.UpdatedBy != nil {
		updatedBy := ConvertUser(*markertype.UpdatedBy)
		item.UpdatedBy = &updatedBy
	}

	return item
}
