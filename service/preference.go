package service

import (
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
)

func CreateRelation(input *dbmodel.UserRelation, user1 dbmodel.User, user2 dbmodel.User) error {
	var relation dbmodel.UserRelation
	relation.UserOne = user1
	relation.UserTwo = user2

	if err := relation.Create(); err != nil {
		return err
	}

	input = &relation

	return nil
}

func UpdateRelation(input model.UpdateRelation, user dbmodel.User) (*dbmodel.UserRelation, error) {
	var preference dbmodel.UserPreference
	preference.CurrentUser = user
	if err := preference.GetOrCreateByUserId(); err != nil {
		return nil, err
	}

	var relatedUser dbmodel.User
	relatedUser.Username = input.Username

	if err := relatedUser.GetUserByUsername(); err != nil {
		if utils.RecordNotFound(err) {
			return nil, &helper.RecordNotFoundError{}
		}
		return nil, err
	}

	var relation dbmodel.UserRelation
	relation.UserOne = user
	relation.UserTwo = relatedUser
	if err := relation.GetOrCreateByUsers(); err != nil {
		return nil, err
	}

	preference.SelectedRelation = &relation
	if err := preference.Update(); err != nil {
		return nil, err
	}

	return &relation, nil
}

func GetCurrentRelation(user dbmodel.User) (*dbmodel.UserRelation, error) {
	var preference dbmodel.UserPreference
	preference.UserId = user.ID
	if err := preference.GetByUserId(); err != nil {
		if utils.RecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if preference.RelationId == nil {
		return nil, nil
	}

	var relation dbmodel.UserRelation
	relation.ID = *preference.RelationId
	if err := relation.GetRelationById(); err != nil {
		if utils.RecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &relation, nil
}
