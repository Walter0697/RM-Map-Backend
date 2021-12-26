package service

import (
	"encoding/json"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
)

func GetAllReleaseNote() ([]dbmodel.ReleaseNote, error) {
	var notes []dbmodel.ReleaseNote

	if err := database.Connection.Find(&notes).Error; err != nil {
		return notes, err
	}

	return notes, nil
}

func GetLatestReleaseNote() (*dbmodel.ReleaseNote, error) {
	var note dbmodel.ReleaseNote
	if err := note.GetLatestRecord(database.Connection); err != nil {
		return nil, err
	}

	return &note, nil
}

func GetReleaseNoteByVersion(input model.ReleaseNoteFilter) (*dbmodel.ReleaseNote, error) {
	var note dbmodel.ReleaseNote
	note.Version = input.Version
	if err := note.GetReleaseNoteByVersion(database.Connection); err != nil {
		return nil, err
	}

	return &note, nil
}

func CheckReleaseNoteAdded(version string) bool {
	var release_note dbmodel.ReleaseNote
	release_note.Version = version

	return release_note.CheckReleaseRecordExist(database.Connection)
}

func CreateReleaseNote(version string, note []string) error {
	var release_note dbmodel.ReleaseNote
	release_note.Version = version

	combined_notes, err := json.Marshal(note)
	if err != nil {
		return err
	}
	release_note.Notes = string(combined_notes)

	if err := release_note.Create(database.Connection); err != nil {
		return err
	}

	return nil
}
