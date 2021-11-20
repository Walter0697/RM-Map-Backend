package service

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
)

func GetAllDefaultPins() ([]dbmodel.DefaultValue, error) {
	var values []dbmodel.DefaultValue

	labelList := constant.GetDefaultPinList()
	// if err := database.Connection.Where("label in ?", labelList).Find(&values).Error; err != nil {
	// 	return values, err
	// }

	for _, label := range labelList {
		var value dbmodel.DefaultValue
		value.Label = label
		if err := value.GetOrCreatePin(database.Connection); err != nil {
			return values, err
		}
		values = append(values, value)
	}

	return values, nil
}

func EditDefaultPin(input model.UpdatedDefault, user dbmodel.User) (*dbmodel.DefaultValue, error) {
	var value dbmodel.DefaultValue
	value.Label = input.Label

	if err := value.GetOrCreatePin(database.Connection); err != nil {
		return nil, err
	}

	var pin dbmodel.Pin
	pin.ID = uint(*input.IntValue)

	if err := pin.GetById(database.Connection); err != nil {
		return nil, helper.GetDatabaseError(err)
	}

	value.PinType = nil
	value.PinId = &pin.ID
	value.UpdatedBy = &user

	if err := value.Update(database.Connection); err != nil {
		return nil, err
	}

	return nil, nil
}
