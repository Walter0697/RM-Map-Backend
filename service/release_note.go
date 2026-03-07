package service

import (
	"encoding/json"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"strings"
	"time"

	"gorm.io/gorm"
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
	_ = normalizeLegacyReleaseNotePublishState()
	notes, err := GetAllReleaseNote()
	if err != nil {
		return nil, err
	}

	if len(notes) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	latest := notes[0]
	latestVersion := normalizeSemver(strings.TrimSpace(latest.Version))
	for _, item := range notes[1:] {
		currentVersion := normalizeSemver(strings.TrimSpace(item.Version))
		switch {
		case currentVersion != "" && latestVersion != "":
			if compareSemver(currentVersion, latestVersion) > 0 {
				latest = item
				latestVersion = currentVersion
			}
		case currentVersion != "" && latestVersion == "":
			latest = item
			latestVersion = currentVersion
		case currentVersion == "" && latestVersion == "":
			if item.CreatedAt.After(latest.CreatedAt) {
				latest = item
			}
		}
	}

	return &latest, nil
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
