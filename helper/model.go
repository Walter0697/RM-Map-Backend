package helper

import (
	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/utils"
)

func ConvertUser(user dbmodel.User) model.User {
	var item model.User
	item.ID = int(user.ID)
	item.Username = user.Username
	item.Role = user.Role
	item.CreatedAt = utils.ConvertToOutputTime(user.CreatedAt)
	return item
}

func ConvertMarker(marker dbmodel.Marker) model.Marker {
	var item model.Marker
	item.ID = int(marker.ID)
	item.Label = marker.Label
	item.Latitude = marker.Latitude
	item.Longitude = marker.Longitude
	item.Address = marker.Address
	item.ImageLink = &marker.ImageLink
	item.Link = &marker.Link
	item.Type = marker.Type
	item.Description = &marker.Description
	item.EstimateTime = &marker.EstimateTime
	item.Price = &marker.Price
	item.Status = &marker.Status

	if marker.FromTime != nil {
		fromTime := utils.ConvertToOutputTime(*marker.FromTime)
		item.FromTime = &fromTime
	}

	if marker.ToTime != nil {
		toTime := utils.ConvertToOutputTime(*marker.ToTime)
		item.ToTime = &toTime
	}

	item.CreatedAt = utils.ConvertToOutputTime(marker.CreatedAt)
	item.UpdatedAt = utils.ConvertToOutputTime(marker.UpdatedAt)
	if marker.CreatedBy != nil {
		createdBy := ConvertUser(*marker.CreatedBy)
		item.CreatedBy = &createdBy
	}
	if marker.UpdatedBy != nil {
		updatedBy := ConvertUser(*marker.UpdatedBy)
		item.UpdatedBy = &updatedBy
	}

	item.IsFav = marker.IsFavourite
	return item
}

func ConvertSchedule(schedule dbmodel.Schedule) model.Schedule {
	var item model.Schedule
	item.ID = int(schedule.ID)
	item.Label = schedule.Label
	item.Description = schedule.Description
	item.Status = schedule.Status
	item.SelectedDate = utils.ConvertToOutputTime(schedule.SelectedDate)
	if schedule.SelectedMarker != nil {
		marker := ConvertMarker(*schedule.SelectedMarker)
		item.Marker = &marker
	}

	item.CreatedAt = utils.ConvertToOutputTime(schedule.CreatedAt)
	item.UpdatedAt = utils.ConvertToOutputTime(schedule.UpdatedAt)
	if schedule.CreatedBy != nil {
		createdBy := ConvertUser(*schedule.CreatedBy)
		item.CreatedBy = &createdBy
	}
	if schedule.UpdatedBy != nil {
		updatedBy := ConvertUser(*schedule.UpdatedBy)
		item.UpdatedBy = &updatedBy
	}

	return item
}

func ConvertMarkerType(markertype dbmodel.MarkerType) model.MarkerType {
	var item model.MarkerType
	item.ID = int(markertype.ID)
	item.Label = markertype.Label
	item.Value = markertype.Value
	item.Priority = markertype.Priority
	item.IconPath = markertype.IconPath

	item.CreatedAt = utils.ConvertToOutputTime(markertype.CreatedAt)
	item.UpdatedAt = utils.ConvertToOutputTime(markertype.UpdatedAt)
	if markertype.CreatedBy != nil {
		createdBy := ConvertUser(*markertype.CreatedBy)
		item.CreatedBy = &createdBy
	}
	if markertype.UpdatedBy != nil {
		updatedBy := ConvertUser(*markertype.UpdatedBy)
		item.UpdatedBy = &updatedBy
	}

	return item
}

func ConvertMarkerTypeToEventType(markertype dbmodel.MarkerType) model.EventType {
	var item model.EventType
	item.Label = markertype.Label
	item.Value = markertype.Value
	item.Priority = markertype.Priority
	item.IconPath = markertype.IconPath

	return item
}

func ConvertPin(pin dbmodel.Pin) model.Pin {
	var item model.Pin
	item.ID = int(pin.ID)
	item.Label = pin.Label
	item.TopLeftX = pin.TopLeftX
	item.TopLeftY = pin.TopLeftY
	item.BottomRightX = pin.BottomRightX
	item.BottomRightY = pin.BottomRightY
	item.ImagePath = pin.DisplayPath
	item.DisplayPath = pin.ImagePath

	item.CreatedAt = utils.ConvertToOutputTime(pin.CreatedAt)
	item.UpdatedAt = utils.ConvertToOutputTime(pin.UpdatedAt)
	if pin.CreatedBy != nil {
		createdBy := ConvertUser(*pin.CreatedBy)
		item.CreatedBy = &createdBy
	}
	if pin.UpdatedBy != nil {
		updatedBy := ConvertUser(*pin.UpdatedBy)
		item.UpdatedBy = &updatedBy
	}

	return item
}

func ConvertToDefaultPin(defaultPin dbmodel.DefaultValue) model.DefaultPin {
	var item model.DefaultPin
	item.Label = defaultPin.Label
	if defaultPin.PinType != nil {
		pin := ConvertPin(*defaultPin.PinType)
		item.Pin = &pin
	}

	if defaultPin.CreatedBy != nil {
		createtime := utils.ConvertToOutputTime(defaultPin.CreatedAt)
		item.CreatedAt = &createtime
		createdBy := ConvertUser(*defaultPin.CreatedBy)
		item.CreatedBy = &createdBy
	}
	if defaultPin.UpdatedBy != nil {
		updatetime := utils.ConvertToOutputTime(defaultPin.UpdatedAt)
		item.UpdatedAt = &updatetime
		updatedBy := ConvertUser(*defaultPin.UpdatedBy)
		item.UpdatedBy = &updatedBy
	}

	return item
}

func UserPreferencePin(preference *dbmodel.UserPreference, default_pins []dbmodel.DefaultValue) model.UserPreference {
	var item model.UserPreference

	if preference != nil {
		for _, pin := range default_pins {
			pinmodel := GetDefaultOrPreferredPin(*preference, pin)
			switch pin.Label {
			case constant.RegularPin:
				item.RegularPin = pinmodel
				break
			case constant.FavouritePin:
				item.FavouritePin = pinmodel
				break
			case constant.SelectedPin:
				item.SelectedPin = pinmodel
				break
			case constant.HurryPin:
				item.HurryPin = pinmodel
				break
			}
		}
	} else {
		for _, pin := range default_pins {
			if pin.PinType != nil {
				pinmodel := ConvertPin(*pin.PinType)
				switch pin.Label {
				case constant.RegularPin:
					item.RegularPin = &pinmodel
					break
				case constant.FavouritePin:
					item.FavouritePin = &pinmodel
					break
				case constant.SelectedPin:
					item.SelectedPin = &pinmodel
					break
				case constant.HurryPin:
					item.HurryPin = &pinmodel
					break
				default:
					break
				}
			}
		}
	}

	return item
}

func GetDefaultOrPreferredPin(preference dbmodel.UserPreference, default_pin dbmodel.DefaultValue) *model.Pin {
	var pin model.Pin
	switch default_pin.Label {
	case constant.RegularPin:
		if preference.RegularPin != nil {
			pin = ConvertPin(*preference.RegularPin)
		} else if default_pin.PinType != nil {
			pin = ConvertPin(*default_pin.PinType)
		} else {
			return nil
		}
		return &pin
	case constant.FavouritePin:
		if preference.FavouritePin != nil {
			pin = ConvertPin(*preference.FavouritePin)
		} else if default_pin.PinType != nil {
			pin = ConvertPin(*default_pin.PinType)
		} else {
			return nil
		}
		return &pin
	case constant.SelectedPin:
		if preference.SelectedPin != nil {
			pin = ConvertPin(*preference.SelectedPin)
		} else if default_pin.PinType != nil {
			pin = ConvertPin(*default_pin.PinType)
		} else {
			return nil
		}
		return &pin
	case constant.HurryPin:
		if preference.HurryPin != nil {
			pin = ConvertPin(*preference.HurryPin)
		} else if default_pin.PinType != nil {
			pin = ConvertPin(*default_pin.PinType)
		} else {
			return nil
		}
		return &pin
	}
	return nil
}

type MapPinRef struct {
	TypePin     dbmodel.TypePin
	MapPinLabel string
}

func ConvertToMapPin(input MapPinRef) model.MapPin {
	var item model.MapPin
	item.Typelabel = input.TypePin.RelatedType.Label
	item.Pinlabel = input.MapPinLabel
	item.ImagePath = input.TypePin.ImagePath

	return item
}

func ConvertPinToMapPin(input dbmodel.Pin, pintype string) model.MapPin {
	var item model.MapPin
	item.Typelabel = ""
	item.Pinlabel = pintype
	item.ImagePath = input.ImagePath

	return item
}
