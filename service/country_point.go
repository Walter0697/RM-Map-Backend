package service

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
	"strings"
	"time"
)

func GetAllCountryPoint(relation dbmodel.UserRelation) ([]dbmodel.CountryPoint, error) {
	var points []dbmodel.CountryPoint
	query := database.Connection

	query = query.Preload("Relation")
	query = query.Where("relation_id = ?", relation.ID)

	if err := query.Find(&points).Error; err != nil {
		return points, err
	}

	return points, nil
}

func GetAllCountryLocation(relation dbmodel.UserRelation) ([]dbmodel.CountryLocation, error) {
	var locations []dbmodel.CountryLocation
	query := database.Connection

	query = query.Preload("Relation")
	query = query.Where("relation_id = ?", relation.ID)

	if err := query.Find(&locations).Error; err != nil {
		return locations, err
	}

	return locations, nil
}

func CreateCountryPoint(input model.NewCountryPoint, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.CountryPoint, error) {
	var point dbmodel.CountryPoint

	point.Label = input.Label
	point.PhotoX = input.PhotoX
	point.PhotoY = input.PhotoY
	point.MapX = input.MapX
	point.MapY = input.MapY
	point.MapName = input.MapName
	point.Relation = relation
	point.CreatedBy = &user
	point.UpdatedBy = &user

	if err := point.Create(database.Connection); err != nil {
		return nil, err
	}

	return &point, nil
}

func CreateCountryLocation(input model.NewCountryLocation, point dbmodel.CountryPoint, marker *dbmodel.Marker, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.CountryLocation, error) {
	var location dbmodel.CountryLocation

	var visitTime time.Time
	var err error

	if input.VisitTime != nil {
		visitTime, err = time.Parse(utils.StandardTime, *input.VisitTime)
		if err != nil {
			return nil, err
		}
	}
	location.Label = input.Label

	imageFileName := ""
	if input.ImageUpload != nil {
		typeInfo := strings.Split(input.ImageUpload.ContentType, "/")
		if typeInfo[0] != "image" {
			return nil, &helper.UploadFileNotImageError{}
		}

		filename := constant.GetImageName(constant.CountryImagePath, typeInfo[1])
		filepath := constant.BasePath + filename
		if err := utils.UploadImage(filepath, input.ImageUpload.File); err != nil {
			return nil, err
		}

		imageFileName = filename
	}

	location.ImageLink = imageFileName

	location.RelatedPoint = point
	location.RelatedMarker = marker
	location.VisitTime = &visitTime

	location.Relation = relation

	location.CreatedBy = &user
	location.UpdatedBy = &user

	if err := location.Create(database.Connection); err != nil {
		return nil, err
	}

	return &location, nil
}
