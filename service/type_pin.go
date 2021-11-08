package service

import (
	"image"
	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"

	"github.com/disintegration/imaging"
)

func CreateTypePin(markertype dbmodel.MarkerType, pin dbmodel.Pin) (dbmodel.TypePin, error) {
	var typePin dbmodel.TypePin

	typePin.TypeId = markertype.ID
	typePin.PinId = pin.ID

	if err := typePin.GetOrCreate(); err != nil {
		return typePin, err
	}

	pinImage, err := imaging.Open(constant.BasePath + pin.ImagePath)
	if err != nil {
		return typePin, err
	}

	typeImage, err := imaging.Open(constant.BasePath + markertype.IconPath)
	if err != nil {
		return typePin, err
	}

	iconWidth := pin.BottomRightX - pin.TopLeftX
	iconHeight := pin.BottomRightY - pin.TopLeftY

	smallTypeIcon := imaging.Resize(typeImage, iconWidth, iconHeight, imaging.Lanczos)

	finalImage := imaging.Overlay(pinImage, smallTypeIcon, image.Pt(pin.TopLeftX, pin.TopLeftY), 1)

	filename := constant.TypePinImageName(int(markertype.ID), int(pin.ID), "png")
	filepath := constant.BasePath + filename

	err = imaging.Save(finalImage, filepath)
	if err != nil {
		return typePin, err
	}

	typePin.ImagePath = filename

	if err := typePin.Update(); err != nil {
		return typePin, err
	}

	return typePin, nil
}

func UpdateTypePinByPin(pin dbmodel.Pin) error {
	// get all the marker types
	var empty []string
	typeList, err := GetAllMarkerType(empty)
	if err != nil {
		return err
	}

	for _, markertype := range typeList {
		_, err := CreateTypePin(markertype, pin)
		if err != nil {
			return err
		}
	}

	return nil
}

func UpdateTypePinByType(markertype dbmodel.MarkerType) error {
	// get all the pins
	var empty []string
	pinList, err := GetAllPin(empty)
	if err != nil {
		return err
	}

	for _, pin := range pinList {
		_, err := CreateTypePin(markertype, pin)
		if err != nil {
			return err
		}
	}

	return nil
}

func FetchAllTypePinByUserPreference(preference model.UserPreference) ([]helper.MapPinRef, error) {
	var list []helper.MapPinRef

	types, err := GetAllEventType()
	if err != nil {
		return list, err
	}

	for _, item := range types {
		if preference.RegularPin != nil {
			var regular dbmodel.TypePin
			regular.PinId = uint(preference.RegularPin.ID)
			regular.TypeId = item.ID
			if err := regular.GetFull(); err != nil {
				return list, err
			}
			var regularRef helper.MapPinRef
			regularRef.TypePin = regular
			regularRef.MapPinLabel = constant.RegularPin
			list = append(list, regularRef)
		}

		if preference.FavouritePin != nil {
			var favourite dbmodel.TypePin
			favourite.PinId = uint(preference.FavouritePin.ID)
			favourite.TypeId = item.ID
			if err := favourite.GetFull(); err != nil {
				return list, err
			}
			var favouriteRef helper.MapPinRef
			favouriteRef.TypePin = favourite
			favouriteRef.MapPinLabel = constant.FavouritePin
			list = append(list, favouriteRef)
		}

		if preference.SelectedPin != nil {
			var selected dbmodel.TypePin
			selected.PinId = uint(preference.SelectedPin.ID)
			selected.TypeId = item.ID
			if err := selected.GetFull(); err != nil {
				return list, err
			}
			var selectedRef helper.MapPinRef
			selectedRef.TypePin = selected
			selectedRef.MapPinLabel = constant.SelectedPin
			list = append(list, selectedRef)
		}

		if preference.HurryPin != nil {
			var hurry dbmodel.TypePin
			hurry.PinId = uint(preference.HurryPin.ID)
			hurry.TypeId = item.ID
			if err := hurry.GetFull(); err != nil {
				return list, err
			}
			var hurryRef helper.MapPinRef
			hurryRef.TypePin = hurry
			hurryRef.MapPinLabel = constant.HurryPin
			list = append(list, hurryRef)
		}
	}

	return list, nil
}
