package service

import (
	"mapmarker/backend/constant"
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

func UpdatePreferredPin(input model.UpdatePreferredPin, user dbmodel.User) (*dbmodel.UserPreference, error) {
	var preference dbmodel.UserPreference
	preference.CurrentUser = user
	if err := preference.GetOrCreateByUserId(); err != nil {
		return nil, err
	}

	if input.PinID != nil {
		var pin dbmodel.Pin
		pin.ID = uint(*input.PinID)
		if err := pin.GetById(); err != nil {
			return nil, err
		}

		switch input.Label {
		case constant.RegularPin:
			preference.RpinId = &pin.ID
			break
		case constant.FavouritePin:
			preference.FpinId = &pin.ID
			break
		case constant.SchedulePin:
			preference.SpinId = &pin.ID
			break
		case constant.HurryPin:
			preference.HpinId = &pin.ID
			break
		default:
			break
		}
	} else {
		switch input.Label {
		case constant.RegularPin:
			preference.RpinId = nil
			break
		case constant.FavouritePin:
			preference.FpinId = nil
			break
		case constant.SchedulePin:
			preference.SpinId = nil
			break
		case constant.HurryPin:
			preference.HpinId = nil
			break
		default:
			break
		}
	}

	if err := preference.Update(); err != nil {
		return nil, err
	}

	return &preference, nil
}
