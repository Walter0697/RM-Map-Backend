package service

import (
	"image"
	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"

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
