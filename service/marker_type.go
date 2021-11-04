package service

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
	"strings"
)

func CreateMarkerType(input model.NewMarkerType, user dbmodel.User) (*dbmodel.MarkerType, error) {
	var markertype dbmodel.MarkerType

	markertype.Label = input.Label
	markertype.Value = input.Value
	markertype.Priority = input.Priority

	if input.IconUpload != nil {
		typeInfo := strings.Split(input.IconUpload.ContentType, "/")
		if typeInfo[0] != "image" {
			return nil, &helper.UploadFileNotImageError{}
		}

		filename := constant.GetImageName(constant.TypeIconPath, typeInfo[1])
		filepath := constant.BasePath + filename
		if err := utils.UploadImage(filepath, input.IconUpload.File); err != nil {
			return nil, err
		}

		markertype.IconPath = filename
	} else {
		return nil, &helper.ImageNotFoundError{}
	}

	markertype.CreatedBy = &user
	markertype.UpdatedBy = &user

	if err := markertype.Create(); err != nil {
		return nil, err
	}

	return &markertype, nil
}

func EditMarkerType(input model.UpdatedMarkerType, user dbmodel.User) (*dbmodel.MarkerType, error) {
	var markertype dbmodel.MarkerType

	markertype.ID = uint(input.ID)
	if err := markertype.GetById(); err != nil {
		return nil, err
	}

	if input.Label != nil {
		markertype.Label = *input.Label
	}

	if input.Value != nil {
		markertype.Value = *input.Value
	}

	if input.Priority != nil {
		markertype.Priority = *input.Priority
	}

	if input.IconUpload != nil {
		typeInfo := strings.Split(input.IconUpload.ContentType, "/")
		if typeInfo[0] != "image" {
			return nil, &helper.UploadFileNotImageError{}
		}

		filename := constant.GetImageName(constant.TypeIconPath, typeInfo[1])
		filepath := constant.BasePath + filename
		if err := utils.UploadImage(filepath, input.IconUpload.File); err != nil {
			return nil, err
		}

		markertype.IconPath = filename
	}

	markertype.UpdatedBy = &user

	if err := markertype.Update(); err != nil {
		return nil, err
	}

	return &markertype, nil
}

func RemoveMarkerType(input model.RemoveModel) error {
	var markertype dbmodel.MarkerType

	markertype.ID = uint(input.ID)

	if err := markertype.RemoveById(); err != nil {
		return err
	}

	return nil
}

func GetAllMarkerType(requested []string) ([]dbmodel.MarkerType, error) {
	var types []dbmodel.MarkerType
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	if err := query.Find(&types).Error; err != nil {
		return types, err
	}

	return types, nil
}

func GetAllEventType() ([]dbmodel.MarkerType, error) {
	var types []dbmodel.MarkerType
	if err := database.Connection.Find(&types).Error; err != nil {
		return types, err
	}

	return types, nil
}

func GetMarkerTypeById(id int) (*dbmodel.MarkerType, error) {
	var markertype dbmodel.MarkerType
	markertype.ID = uint(id)
	if err := markertype.GetById(); err != nil {
		return nil, helper.GetDatabaseError(err)
	}

	return &markertype, nil
}
