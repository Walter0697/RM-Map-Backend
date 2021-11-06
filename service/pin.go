package service

import (
	"image"
	"image/color"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"mapmarker/backend/utils"
	"strings"

	"github.com/disintegration/imaging"
)

type Boundary struct {
	TopLeftX     int
	TopLeftY     int
	BottomRightX int
	BottomRightY int
}

func PreviewPin(input model.PreviewPinInput, markertype dbmodel.MarkerType) (string, error) {
	pinFile, _, err := image.Decode(input.ImageUpload.File)
	if err != nil {
		return "", err
	}

	iconWidth := input.BottomRightX - input.TopLeftX
	iconHeight := input.BottomRightY - input.TopLeftY

	typeImage, err := imaging.Open(constant.BasePath + markertype.IconPath)
	smallTypeIcon := imaging.Resize(typeImage, iconWidth, iconHeight, imaging.Lanczos)

	finalImage := imaging.Overlay(pinFile, smallTypeIcon, image.Pt(input.TopLeftX, input.TopLeftY), 1)

	filename := constant.GetImageName(constant.PreviewImagePath, "png")
	filepath := constant.BasePath + filename

	err = imaging.Save(finalImage, filepath)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func CreateDisplayPin(pinFile image.Image, boundary Boundary) (string, error) {
	iconWidth := boundary.BottomRightX - boundary.TopLeftX
	iconHeight := boundary.BottomRightY - boundary.TopLeftY

	var pixel color.RGBA
	pixel.B = 200
	pixel.A = 0xff

	sampleTypeImage := imaging.New(iconWidth, iconHeight, pixel)

	finalImage := imaging.Overlay(pinFile, sampleTypeImage, image.Pt(boundary.TopLeftX, boundary.TopLeftY), 0.5)

	filename := constant.GetPinDisplayName("png")
	filepath := constant.BasePath + filename

	err := imaging.Save(finalImage, filepath)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func CreatePin(input model.NewPin, user dbmodel.User) (*dbmodel.Pin, error) {
	var pin dbmodel.Pin

	pin.Label = input.Label
	pin.TopLeftX = input.TopLeftX
	pin.TopLeftY = input.TopLeftY
	pin.BottomRightX = input.BottomRightX
	pin.BottomRightY = input.BottomRightY

	if input.ImageUpload != nil {
		typeInfo := strings.Split(input.ImageUpload.ContentType, "/")
		if typeInfo[0] != "image" {
			return nil, &helper.UploadFileNotImageError{}
		}

		// decode the image
		pinFile, _, err := image.Decode(input.ImageUpload.File)
		if err != nil {
			return nil, err
		}

		filename := constant.GetImageName(constant.PinImagePath, typeInfo[1])
		filepath := constant.BasePath + filename

		// clone the image for saving the origin
		pinOriginFile := imaging.Clone(pinFile)
		err = imaging.Save(pinOriginFile, filepath)
		if err != nil {
			return nil, err
		}

		pin.ImagePath = filename

		// then, use the pin file to handle preview image for admin
		var bound Boundary
		bound.TopLeftX = input.TopLeftX
		bound.TopLeftY = input.TopLeftY
		bound.BottomRightX = input.BottomRightX
		bound.BottomRightY = input.BottomRightY

		display_filename, err := CreateDisplayPin(pinFile, bound)
		if err != nil {
			return nil, err
		}

		pin.DisplayPath = display_filename
	}

	pin.CreatedBy = &user
	pin.UpdatedBy = &user

	if err := pin.Create(); err != nil {
		return nil, err
	}

	UpdateTypePinByPin(pin)

	return &pin, nil
}

func EditPin(input model.UpdatedPin, user dbmodel.User) (*dbmodel.Pin, error) {
	var pin dbmodel.Pin

	pin.ID = uint(input.ID)
	if err := pin.GetById(); err != nil {
		return nil, err
	}

	if input.Label != nil {
		pin.Label = *input.Label
	}

	shouldUpdatePosition := false

	if input.TopLeftX != nil {
		pin.TopLeftX = *input.TopLeftX
		shouldUpdatePosition = true
	}

	if input.TopLeftY != nil {
		pin.TopLeftY = *input.TopLeftY
		shouldUpdatePosition = true
	}

	if input.BottomRightX != nil {
		pin.BottomRightX = *input.BottomRightX
		shouldUpdatePosition = true
	}

	if input.BottomRightY != nil {
		pin.BottomRightY = *input.BottomRightY
		shouldUpdatePosition = true
	}

	if input.ImageUpload != nil {
		typeInfo := strings.Split(input.ImageUpload.ContentType, "/")
		if typeInfo[0] != "image" {
			return nil, &helper.UploadFileNotImageError{}
		}

		// decode the image
		pinFile, _, err := image.Decode(input.ImageUpload.File)
		if err != nil {
			return nil, err
		}

		filename := constant.GetImageName(constant.PinImagePath, typeInfo[1])
		filepath := constant.BasePath + filename

		// clone the image for saving the origin
		pinOriginFile := imaging.Clone(pinFile)
		err = imaging.Save(pinOriginFile, filepath)
		if err != nil {
			return nil, err
		}

		pin.ImagePath = filename

		var bound Boundary
		bound.TopLeftX = pin.TopLeftX
		bound.TopLeftY = pin.TopLeftY
		bound.BottomRightX = pin.BottomRightX
		bound.BottomRightY = pin.BottomRightY

		display_filename, err := CreateDisplayPin(pinFile, bound)
		if err != nil {
			return nil, err
		}

		pin.DisplayPath = display_filename
	} else if shouldUpdatePosition {
		pinFile, err := imaging.Open(constant.BasePath + pin.ImagePath)

		var bound Boundary
		bound.TopLeftX = pin.TopLeftX
		bound.TopLeftY = pin.TopLeftY
		bound.BottomRightX = pin.BottomRightX
		bound.BottomRightY = pin.BottomRightY

		display_filename, err := CreateDisplayPin(pinFile, bound)
		if err != nil {
			return nil, err
		}

		pin.DisplayPath = display_filename
	}

	pin.UpdatedBy = &user

	if err := pin.Update(); err != nil {
		return nil, err
	}

	UpdateTypePinByPin(pin)

	return &pin, nil
}

func RemovePin(input model.RemoveModel) error {
	var pin dbmodel.Pin

	pin.ID = uint(input.ID)

	if err := pin.RemoveById(); err != nil {
		return err
	}

	return nil
}

func GetAllPin(requested []string) ([]dbmodel.Pin, error) {
	var pins []dbmodel.Pin
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}

	if err := query.Find(&pins).Error; err != nil {
		return pins, err
	}

	return pins, nil
}
