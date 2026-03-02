package service

import (
	"fmt"
	"strings"

	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
)

func ResolveTypePinImagePath(pinID uint, markerTypeName string) (string, error) {
	trimmedType := strings.TrimSpace(markerTypeName)
	if trimmedType == "" {
		return "", fmt.Errorf("marker_type_name is required")
	}

	var markerType dbmodel.MarkerType
	if err := database.Connection.
		Where("LOWER(label) = LOWER(?) OR LOWER(value) = LOWER(?)", trimmedType, trimmedType).
		First(&markerType).Error; err != nil {
		return "", ErrUnknownMarkerType
	}

	var typePin dbmodel.TypePin
	typePin.PinId = pinID
	typePin.TypeId = markerType.ID
	if err := typePin.GetFull(database.Connection); err != nil {
		return "", ErrTypePinMissing
	}

	imagePath := strings.TrimSpace(typePin.ImagePath)
	if imagePath == "" {
		return "", ErrTypePinMissing
	}

	return imagePath, nil
}
