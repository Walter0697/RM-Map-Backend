package service

import (
	"errors"
	"fmt"
	"strings"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
)

var (
	ErrUnknownUsername             = errors.New("unknown username")
	ErrPreviewPinSelectionRequired = errors.New("preview pin selection is required")
	ErrPreviewPinInvalid           = errors.New("preview pin selection is invalid")
)

var getUserPreviewPinSelectionFn = GetUserPreviewPinSelection
var getDefaultPinByLabelFn = GetDefaultPinByLabel
var getPinByIDFn = func(pinID uint) (*dbmodel.Pin, error) {
	var pin dbmodel.Pin
	pin.ID = pinID
	if err := pin.GetById(database.Connection); err != nil {
		return nil, err
	}
	return &pin, nil
}

func SetUserPreviewPinSelection(username string, pinID uint, actor *dbmodel.User) (*dbmodel.UserPreference, *dbmodel.Pin, error) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return nil, nil, fmt.Errorf("username is required")
	}
	if pinID == 0 {
		return nil, nil, fmt.Errorf("pin_id is required")
	}

	var user dbmodel.User
	user.Username = trimmedUsername
	if err := user.GetUserByUsername(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return nil, nil, ErrUnknownUsername
		}
		return nil, nil, err
	}

	var pin dbmodel.Pin
	pin.ID = pinID
	if err := pin.GetById(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return nil, nil, ErrPreviewPinInvalid
		}
		return nil, nil, err
	}

	preference := dbmodel.UserPreference{CurrentUser: user}
	if err := preference.GetOrCreateByUserId(database.Connection); err != nil {
		return nil, nil, err
	}

	preference.PreviewPin = &pin
	preference.PreviewPinID = &pin.ID
	_ = actor
	if err := preference.Update(database.Connection); err != nil {
		return nil, nil, err
	}

	return &preference, &pin, nil
}

func GetUserPreviewPinSelection(username string) (*dbmodel.User, *dbmodel.UserPreference, error) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return nil, nil, fmt.Errorf("username is required")
	}

	var user dbmodel.User
	user.Username = trimmedUsername
	if err := user.GetUserByUsername(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return nil, nil, ErrUnknownUsername
		}
		return nil, nil, err
	}

	preference := dbmodel.UserPreference{UserId: user.ID}
	if err := preference.GetByUserId(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return &user, nil, nil
		}
		return nil, nil, err
	}

	return &user, &preference, nil
}

func ResolveUserPreviewPinByUsername(username string) (*dbmodel.User, *dbmodel.Pin, error) {
	user, preference, err := getUserPreviewPinSelectionFn(username)
	if err != nil {
		return nil, nil, err
	}
	if preference == nil || preference.PreviewPinID == nil || *preference.PreviewPinID == 0 {
		defaultPreviewPin, defaultErr := getDefaultPinByLabelFn(constant.PreviewPin)
		if defaultErr != nil {
			return nil, nil, defaultErr
		}
		if defaultPreviewPin.PinId == nil || *defaultPreviewPin.PinId == 0 || defaultPreviewPin.PinType == nil {
			return nil, nil, ErrPreviewPinSelectionRequired
		}
		return user, defaultPreviewPin.PinType, nil
	}
	if preference.PreviewPin != nil {
		return user, preference.PreviewPin, nil
	}

	pin, err := getPinByIDFn(*preference.PreviewPinID)
	if err != nil {
		if utils.RecordNotFound(err) {
			return nil, nil, ErrPreviewPinInvalid
		}
		return nil, nil, err
	}

	return user, pin, nil
}
