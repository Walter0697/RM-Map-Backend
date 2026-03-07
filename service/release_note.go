package service

import (
	"encoding/json"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"strings"
	"time"
)

func GetAllReleaseNote() ([]dbmodel.ReleaseNote, error) {
	var notes []dbmodel.ReleaseNote
	_ = normalizeLegacyReleaseNotePublishState()

	if err := database.Connection.Where("publish_state = ? OR publish_state = ''", "published").Order("published_at desc, created_at desc").Find(&notes).Error; err != nil {
		return notes, err
	}

	return notes, nil
}

func GetLatestReleaseNote() (*dbmodel.ReleaseNote, error) {
	var note dbmodel.ReleaseNote
	_ = normalizeLegacyReleaseNotePublishState()
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

func CreateReleaseNote(version string, note []string, icon *string) error {
	var release_note dbmodel.ReleaseNote
	release_note.Version = version
	release_note.Icon = icon
	release_note.Title = "Release " + strings.TrimSpace(version)
	release_note.ContentFormat = releaseNoteFormatMarkdown
	release_note.NotesFormat = releaseNoteNotesFormatJSON
	release_note.PublishState = releaseNoteStatePublished

	combined_notes, err := json.Marshal(note)
	if err != nil {
		return err
	}
	release_note.Notes = string(combined_notes)
	release_note.Content = notesListToMarkdown(note)
	release_note.SanitizedContent = renderMarkdownAsSafeHTML(release_note.Content)
	if icon != nil && strings.TrimSpace(*icon) != "" {
		refs, _ := json.Marshal([]string{strings.TrimSpace(*icon)})
		release_note.ImageRefs = string(refs)
	}
	now := time.Now().UTC()
	release_note.PublishedAt = &now

	if err := release_note.Create(database.Connection); err != nil {
		return err
	}

	return nil
}
