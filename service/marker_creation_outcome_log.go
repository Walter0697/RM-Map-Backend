package service

import (
	"strings"

	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
)

type MarkerCreationOutcomeLogCreateInput struct {
	Link           string
	Status         string
	MarkerID       *uint
	ExternalRunID  *string
	FailureReason  *string
	FailureMessage *string
}

func CreateMarkerCreationOutcomeLog(input MarkerCreationOutcomeLogCreateInput) (*dbmodel.MarkerCreationOutcomeLog, error) {
	item := &dbmodel.MarkerCreationOutcomeLog{
		Link:           strings.TrimSpace(input.Link),
		Status:         strings.ToLower(strings.TrimSpace(input.Status)),
		MarkerID:       input.MarkerID,
		ExternalRunID:  normalizedOptionalString(input.ExternalRunID),
		FailureReason:  normalizedOptionalString(input.FailureReason),
		FailureMessage: normalizedOptionalString(input.FailureMessage),
	}
	if err := item.Create(database.Connection); err != nil {
		return nil, err
	}
	return item, nil
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
