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
	return item
}
