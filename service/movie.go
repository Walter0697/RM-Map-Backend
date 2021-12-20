package service

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"

	"gorm.io/gorm"
)

func CreateMovie(tx *gorm.DB, input model.NewMovieSchedule, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.Movie, error) {
	var movie dbmodel.Movie

	movie.Label = input.Label
	if input.MovieRelease != nil {
		movie.ReleaseDate = input.MovieRelease
	}

	if input.MovieImage != nil {
		filename := constant.GetImageLinkName(constant.MovieImagePath, *input.MovieImage)
		filepath := constant.BasePath + filename
		if err := utils.SaveImageFromURL(filepath, *input.MovieImage); err != nil {
			return nil, err
		}

		movie.ImageLink = filename
	}

	movie.Relation = relation
	movie.CreatedBy = &user
	movie.UpdatedBy = &user

	if err := movie.Create(tx); err != nil {
		return nil, err
	}

	return &movie, nil
}
