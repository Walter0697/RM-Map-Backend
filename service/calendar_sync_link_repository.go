package service

import (
	"errors"
	"fmt"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrCalendarSyncLinkNotFound          = errors.New("calendar sync link not found")
	ErrCalendarSyncStatusTransition      = errors.New("invalid calendar sync status transition")
	ErrCalendarSyncLinkConnectionMissing = errors.New("calendar sync link connection is required")
)

type CalendarSyncLinkRepository struct {
	db  *gorm.DB
	now func() time.Time
}

type CalendarSyncLinkUpsertInput struct {
	ScheduleID   uint
	ProviderKey  string
	ConnectionID uint
}

func NewCalendarSyncLinkRepository(db *gorm.DB) *CalendarSyncLinkRepository {
	return &CalendarSyncLinkRepository{
		db:  db,
		now: time.Now,
	}
}

func (repository *CalendarSyncLinkRepository) GetByScheduleAndProvider(scheduleID uint, providerKey string) (*dbmodel.ScheduleCalendarSyncLink, error) {
	link := &dbmodel.ScheduleCalendarSyncLink{}
	err := repository.db.
		Where("schedule_id = ? AND provider_key = ?", scheduleID, normalizeCalendarProviderKey(providerKey)).
		First(link).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCalendarSyncLinkNotFound
		}
		return nil, err
	}
	return link, nil
}

func (repository *CalendarSyncLinkRepository) UpsertLink(input CalendarSyncLinkUpsertInput) (*dbmodel.ScheduleCalendarSyncLink, error) {
	if input.ConnectionID == 0 {
		return nil, ErrCalendarSyncLinkConnectionMissing
	}
	link, err := repository.GetByScheduleAndProvider(input.ScheduleID, input.ProviderKey)
	if err != nil && !errors.Is(err, ErrCalendarSyncLinkNotFound) {
		return nil, err
	}
	if errors.Is(err, ErrCalendarSyncLinkNotFound) {
		link = &dbmodel.ScheduleCalendarSyncLink{
			ScheduleID:   input.ScheduleID,
			ProviderKey:  normalizeCalendarProviderKey(input.ProviderKey),
			ConnectionID: input.ConnectionID,
			SyncStatus:   dbmodel.CalendarSyncStatusPending,
		}
		if err := link.Create(repository.db); err != nil {
			return nil, err
		}
		return link, nil
	}
	connectionChanged := link.ConnectionID != input.ConnectionID
	link.ConnectionID = input.ConnectionID
	if connectionChanged {
		link.ExternalEventID = ""
		link.ExternalCalendarID = ""
		link.LastSyncedAt = nil
		link.LastOperationKey = ""
		link.DisconnectedAt = nil
	}
	if err := repository.transitionStatus(link, dbmodel.CalendarSyncStatusPending); err != nil {
		return nil, err
	}
	link.LastErrorCode = ""
	link.LastErrorMessage = ""
	link.RetryCount = 0
	link.NextRetryAt = nil
	return link, nil
}

func (repository *CalendarSyncLinkRepository) MarkSynced(linkID uint, externalEventID string, externalCalendarID string) error {
	link, err := repository.getByID(linkID)
	if err != nil {
		return err
	}
	if err := repository.transitionStatus(link, dbmodel.CalendarSyncStatusSynced); err != nil {
		return err
	}
	now := repository.now().UTC()
	link.ExternalEventID = strings.TrimSpace(externalEventID)
	link.ExternalCalendarID = strings.TrimSpace(externalCalendarID)
	link.LastSyncedAt = &now
	link.LastErrorCode = ""
	link.LastErrorMessage = ""
	link.RetryCount = 0
	link.NextRetryAt = nil
	return link.Update(repository.db)
}

func (repository *CalendarSyncLinkRepository) MarkFailed(linkID uint, errorCode string, errorMessage string, nextRetryAt *time.Time) error {
	link, err := repository.getByID(linkID)
	if err != nil {
		return err
	}
	if err := repository.transitionStatus(link, dbmodel.CalendarSyncStatusFailed); err != nil {
		return err
	}
	link.LastErrorCode = strings.TrimSpace(errorCode)
	link.LastErrorMessage = strings.TrimSpace(errorMessage)
	link.RetryCount = link.RetryCount + 1
	link.NextRetryAt = nextRetryAt
	return link.Update(repository.db)
}

func (repository *CalendarSyncLinkRepository) MarkDisconnected(linkID uint) error {
	link, err := repository.getByID(linkID)
	if err != nil {
		return err
	}
	if err := repository.transitionStatus(link, dbmodel.CalendarSyncStatusDisconnected); err != nil {
		return err
	}
	now := repository.now().UTC()
	link.DisconnectedAt = &now
	link.ExternalEventID = ""
	link.ExternalCalendarID = ""
	link.LastOperationKey = ""
	link.NextRetryAt = nil
	return link.Update(repository.db)
}

func (repository *CalendarSyncLinkRepository) MarkPending(linkID uint) error {
	link, err := repository.getByID(linkID)
	if err != nil {
		return err
	}
	if err := repository.transitionStatus(link, dbmodel.CalendarSyncStatusPending); err != nil {
		return err
	}
	return link.Update(repository.db)
}

func (repository *CalendarSyncLinkRepository) transitionStatus(link *dbmodel.ScheduleCalendarSyncLink, to string) error {
	from := strings.TrimSpace(link.SyncStatus)
	to = strings.TrimSpace(to)
	if from == "" {
		from = dbmodel.CalendarSyncStatusPending
	}
	if !canTransitionCalendarSyncStatus(from, to) {
		return fmt.Errorf("%w: %s -> %s", ErrCalendarSyncStatusTransition, from, to)
	}
	link.SyncStatus = to
	return nil
}

func canTransitionCalendarSyncStatus(from string, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case dbmodel.CalendarSyncStatusPending:
		return to == dbmodel.CalendarSyncStatusSynced || to == dbmodel.CalendarSyncStatusFailed || to == dbmodel.CalendarSyncStatusDisconnected
	case dbmodel.CalendarSyncStatusSynced:
		return to == dbmodel.CalendarSyncStatusPending || to == dbmodel.CalendarSyncStatusFailed || to == dbmodel.CalendarSyncStatusDisconnected
	case dbmodel.CalendarSyncStatusFailed:
		return to == dbmodel.CalendarSyncStatusPending || to == dbmodel.CalendarSyncStatusDisconnected
	case dbmodel.CalendarSyncStatusDisconnected:
		return to == dbmodel.CalendarSyncStatusPending || to == dbmodel.CalendarSyncStatusFailed
	default:
		return false
	}
}

func (repository *CalendarSyncLinkRepository) getByID(linkID uint) (*dbmodel.ScheduleCalendarSyncLink, error) {
	link := &dbmodel.ScheduleCalendarSyncLink{}
	if err := repository.db.Where("id = ?", linkID).First(link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCalendarSyncLinkNotFound
		}
		return nil, err
	}
	return link, nil
}
