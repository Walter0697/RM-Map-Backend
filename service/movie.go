package service

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"

	"gorm.io/gorm"
)

func CreateMovie(tx *gorm.DB, movie_rid int, is_fav bool, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.Movie, error) {
	input_movie, err := FetchMovieByRid(int64(movie_rid))
	if err != nil {
		return nil, err
	}

	var movie dbmodel.Movie

	movie.RefId = movie_rid
	movie.Label = input_movie.Label
	movie.ReleaseDate = input_movie.ReleaseDate
	movie.IsFav = is_fav

	if input_movie.ImageLink != "" {
		filename := constant.GetImageLinkName(constant.MovieImagePath, input_movie.ImageLink)
		filepath := constant.BasePath + filename
		if err := utils.SaveImageFromURL(filepath, input_movie.ImageLink); err != nil {
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

func RemoveFavouriteMovie(tx *gorm.DB, movie_id uint, relation dbmodel.UserRelation, user dbmodel.User) (*dbmodel.Movie, error) {
	var movie dbmodel.Movie

	movie.ID = movie_id
	if err := movie.GetById(tx); err != nil {
		return nil, err
	}

	if movie.RelationId != relation.ID {
		return nil, &helper.InvalidRelationUpdateError{}
	}

	movie.IsFav = false
	movie.UpdatedBy = &user

	if err := movie.Update(tx); err != nil {
		return nil, err
	}

	return &movie, nil
}

func GetMovieByRid(movie_rid int, relation dbmodel.UserRelation) (*dbmodel.Movie, error) {
	var movie dbmodel.Movie

	movie.RefId = movie_rid
	movie.RelationId = relation.ID
	if err := movie.GetByRid(database.Connection); err != nil {
		return nil, err
	}

	return &movie, nil
}

func GetAllMovie(requested []string, relation dbmodel.UserRelation) ([]dbmodel.Movie, error) {
	var movies []dbmodel.Movie
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	query = query.Where("relation_id = ?", relation.ID)

	if err := query.Find(&movies).Error; err != nil {
		return movies, err
	}

	return movies, nil
}