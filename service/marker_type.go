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
