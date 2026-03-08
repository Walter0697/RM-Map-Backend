package service

import (
	"errors"
	"fmt"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCalendarSyncJobNotFound = errors.New("calendar sync job not found")

type CalendarSyncJobQueue struct {
	db  *gorm.DB
	now func() time.Time
}

type CalendarSyncJobEnqueueInput struct {
	ScheduleID     uint
	LinkID         *uint
	ProviderKey    string
	Action         string
	IdempotencyKey string
	PayloadJSON    string
	MaxAttempts    int
}

func NewCalendarSyncJobQueue(db *gorm.DB) *CalendarSyncJobQueue {
	return &CalendarSyncJobQueue{
		db:  db,
		now: time.Now,
	}
}

func (queue *CalendarSyncJobQueue) Enqueue(input CalendarSyncJobEnqueueInput) (*dbmodel.CalendarSyncJob, error) {
	maxAttempts := input.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	now := queue.now().UTC()
	job := &dbmodel.CalendarSyncJob{
		ScheduleID:     input.ScheduleID,
		LinkID:         input.LinkID,
		ProviderKey:    normalizeCalendarProviderKey(input.ProviderKey),
		Action:         strings.TrimSpace(input.Action),
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		Status:         dbmodel.CalendarSyncJobStatusPending,
		PayloadJSON:    strings.TrimSpace(input.PayloadJSON),
		MaxAttempts:    maxAttempts,
		NextAttemptAt:  &now,
	}
	if err := queue.db.Where("idempotency_key = ?", job.IdempotencyKey).FirstOrCreate(job).Error; err != nil {
		return nil, err
	}
	calendarMetricJobQueued()
	return job, nil
}

func (queue *CalendarSyncJobQueue) ClaimDueJobs(limit int) ([]dbmodel.CalendarSyncJob, error) {
	if limit <= 0 {
		limit = 10
	}
	now := queue.now().UTC()
	jobs := make([]dbmodel.CalendarSyncJob, 0, limit)
	err := queue.db.Transaction(func(tx *gorm.DB) error {
		query := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", []string{dbmodel.CalendarSyncJobStatusPending, dbmodel.CalendarSyncJobStatusFailed}, now).
			Order("next_attempt_at asc, id asc").
			Limit(limit)
		if err := query.Find(&jobs).Error; err != nil {
			return err
		}
		if len(jobs) == 0 {
			return nil
		}
		for idx := range jobs {
			jobs[idx].Status = dbmodel.CalendarSyncJobStatusProcessing
			jobs[idx].Attempts = jobs[idx].Attempts + 1
			jobs[idx].StartedAt = &now
			jobs[idx].NextAttemptAt = nil
			if err := tx.Save(&jobs[idx]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	calendarMetricJobClaimed(len(jobs))
	return jobs, err
}

func (queue *CalendarSyncJobQueue) MarkSucceeded(jobID uint) error {
	job, err := queue.getByID(jobID)
	if err != nil {
		return err
	}
	now := queue.now().UTC()
	job.Status = dbmodel.CalendarSyncJobStatusSucceeded
	job.CompletedAt = &now
	job.LastErrorCode = ""
	job.LastErrorDetail = ""
	job.NextAttemptAt = nil
	return job.Update(queue.db)
}

func (queue *CalendarSyncJobQueue) MarkFailed(jobID uint, errorCode string, errorDetail string) error {
	job, err := queue.getByID(jobID)
	if err != nil {
		return err
	}
	job.LastErrorCode = strings.TrimSpace(errorCode)
	job.LastErrorDetail = strings.TrimSpace(errorDetail)
	now := queue.now().UTC()
	if job.Attempts >= job.MaxAttempts {
		job.Status = dbmodel.CalendarSyncJobStatusDead
		job.CompletedAt = &now
		job.NextAttemptAt = nil
		return job.Update(queue.db)
	}
	job.Status = dbmodel.CalendarSyncJobStatusFailed
	next := now.Add(calendarSyncRetryBackoff(job.Attempts))
	job.NextAttemptAt = &next
	return job.Update(queue.db)
}

func calendarSyncRetryBackoff(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return time.Minute
	case attempt == 2:
		return 2 * time.Minute
	case attempt == 3:
		return 5 * time.Minute
	case attempt == 4:
		return 15 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func (queue *CalendarSyncJobQueue) getByID(jobID uint) (*dbmodel.CalendarSyncJob, error) {
	job := &dbmodel.CalendarSyncJob{}
	if err := queue.db.Where("id = ?", jobID).First(job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCalendarSyncJobNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(job.IdempotencyKey) == "" {
		return nil, fmt.Errorf("%w: missing idempotency key", ErrCalendarSyncJobNotFound)
	}
	return job, nil
}
