package service

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

func CreateMarker(input model.NewMarker, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.Marker, error) {
	var fromTime time.Time
	var toTime time.Time
	var err error
	if input.FromTime != nil {
		fromTime, err = time.Parse(utils.StandardTime, *input.FromTime)
		if err != nil {
			return nil, err
		}
	}
	if input.ToTime != nil {
		toTime, err = time.Parse(utils.StandardTime, *input.ToTime)
		if err != nil {
			return nil, err
		}
	}

	imageFileName := ""

	if input.ImageLink != nil {
		extension := filepath.Ext(*input.ImageLink)
		// TODO: for now we use frontend to validate if it is an image
		// maybe add that to backend in the future

		filename := constant.GetImageName(constant.MarkerPreviewPath, extension)
		filepath := constant.BasePath + filename
		if err := utils.SaveImageFromURL(filepath, *input.ImageLink); err != nil {
			return nil, err
		}

		imageFileName = filename
	}

	if input.ImageUpload != nil {
		typeInfo := strings.Split(input.ImageUpload.ContentType, "/")
		if typeInfo[0] != "image" {
			return nil, &helper.UploadFileNotImageError{}
		}

		filename := constant.GetImageName(constant.MarkerPreviewPath, typeInfo[1])
		filepath := constant.BasePath + filename
		if err := utils.UploadImage(filepath, input.ImageUpload.File); err != nil {
			return nil, err
		}

		imageFileName = filename
	}

	var marker dbmodel.Marker

	marker.Label = input.Label
	marker.Latitude = input.Latitude
	marker.Longitude = input.Longitude
	marker.Address = input.Address
	if input.ImageUpload != nil {
		marker.ImageLink = imageFileName
	}
	if input.Link != nil {
		marker.Link = *input.Link
	}
	marker.Type = input.Type
	if input.Description != nil {
		marker.Description = *input.Description
	}
	if input.EstimateTime != nil {
		marker.EstimateTime = *input.EstimateTime
	}
	if input.Price != nil {
		marker.Price = *input.Price
	}
	if input.FromTime != nil {
		marker.FromTime = &fromTime
	}
	if input.ToTime != nil {
		marker.ToTime = &toTime
	}

	marker.Relation = relation

	marker.Status = ""

	marker.CreatedBy = &user
	marker.UpdatedBy = &user

	marker.IsFavourite = false

	if err := marker.Create(database.Connection); err != nil {
		return nil, err
	}

	return &marker, nil
}

func UpdateMarkerFavourite(input model.UpdateMarkerFavourite, user dbmodel.User) (*dbmodel.Marker, error) {
	var marker dbmodel.Marker
	marker.ID = uint(input.ID)
	if err := marker.GetById(database.Connection); err != nil {
		return nil, helper.GetDatabaseError(err)
	}

	marker.IsFavourite = input.IsFav
	marker.UpdatedBy = &user
	marker.UpdatedAt = time.Now()

	if err := marker.Update(database.Connection); err != nil {
		return nil, err
	}

	return &marker, nil
}

func ResetMarkerBySchedule(tx *gorm.DB, input model.RemoveModel, relation dbmodel.UserRelation, user dbmodel.User) (*dbmodel.Marker, error) {
	var schedule dbmodel.Schedule

	schedule.ID = uint(input.ID)
	if err := schedule.GetById(tx); err != nil {
		return nil, err
	}

	if schedule.RelationId != relation.ID {
		return nil, &helper.InvalidRelationUpdateError{}
	}

	schedule.SelectedMarker.Status = ""
	if err := schedule.SelectedMarker.Update(tx); err != nil {
		return nil, err
	}

	return schedule.SelectedMarker, nil
}

func RevokeMarker(input model.UpdateModel, user dbmodel.User) (*dbmodel.Marker, error) {
	var marker dbmodel.Marker
	marker.ID = uint(input.ID)
	if err := marker.GetById(database.Connection); err != nil {
		return nil, helper.GetDatabaseError(err)
	}

	marker.Status = ""
	marker.UpdatedBy = &user
	marker.UpdatedAt = time.Now()

	if err := marker.Update(database.Connection); err != nil {
		return nil, err
	}

	return &marker, nil
}

func GetAllActiveMarker(requested []string, relation dbmodel.UserRelation) ([]dbmodel.Marker, error) {
	var markers []dbmodel.Marker
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	query = query.Where("relation_id = ?", relation.ID)

	// filtering non-active markers
	current := time.Now().AddDate(0, 0, -1) // minus one to make sure time zone won't affect the funcionality
	query = query.Where("status != ? AND status != ?", constant.Arrived, constant.Scheduled)
	query = query.Where("to_time IS NULL OR (to_time IS NOT NULL AND to_time >= ?)", current.Format(time.RFC3339))

	if err := query.Find(&markers).Error; err != nil {
		return markers, err
	}
	return markers, nil
}

func GetAllPreviousMarker(requested []string, relation dbmodel.UserRelation) ([]dbmodel.Marker, error) {
	var markers []dbmodel.Marker
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	query = query.Where("relation_id = ?", relation.ID)

	// filtering active markers
	query = query.Where("status = ?", constant.Arrived)

	if err := query.Find(&markers).Error; err != nil {
		return markers, err
	}
	return markers, nil
}
