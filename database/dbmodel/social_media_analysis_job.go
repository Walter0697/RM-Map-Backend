package dbmodel

import "time"

const (
	SocialMediaAnalysisJobStatusQueued               = "queued"
	SocialMediaAnalysisJobStatusProcessing           = "processing"
	SocialMediaAnalysisJobStatusAwaitingConfirmation = "awaiting_confirmation"
	SocialMediaAnalysisJobStatusCompleted            = "completed"
	SocialMediaAnalysisJobStatusCreated              = "created"
	SocialMediaAnalysisJobStatusNeedsInput           = "needs_input"
	SocialMediaAnalysisJobStatusFailed               = "failed"
	SocialMediaAnalysisJobStatusCancelled            = "cancelled"
)

type SocialMediaAnalysisJob struct {
	BaseModel
	JobID           string     `json:"job_id" gorm:"size:64;not null;uniqueIndex"`
	RelationID      uint       `json:"relation_id" gorm:"not null;index"`
	SourceURL       string     `json:"source_url" gorm:"type:text;not null"`
	SourceJSON      string     `json:"-" gorm:"type:text"`
	Status          string     `json:"status" gorm:"size:32;not null;index"`
	Stage           string     `json:"stage" gorm:"size:64;not null;index"`
	RunnerJobID     string     `json:"runner_job_id,omitempty" gorm:"size:128;index"`
	TraceParent     string     `json:"-" gorm:"size:255"`
	CreatedMarkerID *uint      `json:"created_marker_id,omitempty"`
	ResultJSON      string     `json:"result_json,omitempty" gorm:"type:text"`
	ErrorCode       string     `json:"error_code,omitempty" gorm:"size:96"`
	ErrorDetail     string     `json:"error_detail,omitempty" gorm:"type:text"`
	Attempts        int        `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts     int        `json:"max_attempts" gorm:"not null;default:3"`
	NextAttemptAt   *time.Time `json:"next_attempt_at,omitempty" gorm:"index"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

func NewSocialMediaAnalysisJob(sourceURL string) *SocialMediaAnalysisJob {
	return &SocialMediaAnalysisJob{
		SourceURL:   sourceURL,
		Status:      SocialMediaAnalysisJobStatusQueued,
		Stage:       SocialMediaAnalysisJobStatusQueued,
		MaxAttempts: 3,
	}
}
