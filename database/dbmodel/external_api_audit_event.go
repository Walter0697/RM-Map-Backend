package dbmodel

import (
	"time"

	"gorm.io/gorm"
)

const (
	ExternalAPIAuditStatusSuccess = "success"
	ExternalAPIAuditStatusError   = "error"
)

type ExternalAPIAuditEvent struct {
	BaseModel
	Provider    string     `json:"provider" gorm:"index;not null"`
	Operation   string     `json:"operation" gorm:"index;not null"`
	StatusClass string     `json:"status_class" gorm:"index;not null"`
	HTTPStatus  *int       `json:"http_status"`
	LatencyMS   int64      `json:"latency_ms"`
	RequestTime time.Time  `json:"request_time" gorm:"index;not null"`
	ErrorClass  string     `json:"error_class"`
	ErrorCode   string     `json:"error_code"`
	ErrorDetail string     `json:"error_detail"`
	CompletedAt *time.Time `json:"completed_at"`
}

func (event *ExternalAPIAuditEvent) Create(db *gorm.DB) error {
	return db.Create(event).Error
}
