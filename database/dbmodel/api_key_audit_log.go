package dbmodel

import "gorm.io/gorm"

type APIKeyAuditLog struct {
	BaseModel
	APIKeyName string `json:"api_key_name"`
	APIKeyID   *uint  `json:"api_key_id" gorm:"index"`
	SourceIP   string `json:"source_ip"`
	Operation  string `json:"operation" gorm:"index"`
	Success    bool   `json:"success"`
	Reason     string `json:"reason"`
}

func (audit *APIKeyAuditLog) Create(db *gorm.DB) error {
	return db.Create(audit).Error
}
