package dbmodel

import (
	"log"
	"mapmarker/backend/database"
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uint           `json:"id" gorm:"primary_key"`
	CreatedAt time.Time      `json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type ObjectBase struct {
	BaseModel
	CreatedBy  *User `gorm:"foreignKey:created_uid;references:id"`
	CreatedUID *uint
	UpdatedAt  time.Time `json:"updatedAt"`
	UpdatedBy  *User     `gorm:"foreignKey:updated_uid;references:id"`
	UpdatedUID *uint
}

// note: we don't really need to put json tag in dbmodel but I did it anyway
func AutoMigration() {
	database.Connection.AutoMigrate(&ReleaseNote{})
	database.Connection.AutoMigrate(&DataRecord{})
	database.Connection.AutoMigrate(&User{})
	database.Connection.AutoMigrate(&APIKey{})
	database.Connection.AutoMigrate(&APIKeyAuditLog{})
	database.Connection.AutoMigrate(&ExternalAPIAuditEvent{})
	database.Connection.AutoMigrate(&MarkerCreationOutcomeLog{})
	database.Connection.AutoMigrate(&AdminCleanupJob{})
	database.Connection.AutoMigrate(&UserRelation{})
	database.Connection.AutoMigrate(&UserPreference{})
	database.Connection.AutoMigrate(&Marker{})
	database.Connection.AutoMigrate(&MarkerType{})
	database.Connection.AutoMigrate(&Pin{})
	database.Connection.AutoMigrate(&TypePin{})
	database.Connection.AutoMigrate(&DefaultValue{})
	database.Connection.AutoMigrate(&SystemSetting{})
	database.Connection.AutoMigrate(&Schedule{})
	database.Connection.AutoMigrate(&Movie{})
	database.Connection.AutoMigrate(&Restaurant{})
	database.Connection.AutoMigrate(&TrainStation{})
	database.Connection.AutoMigrate(&TrainStationMap{})
	database.Connection.AutoMigrate(&TrainStationLine{})
	database.Connection.AutoMigrate(&TrainStationStationLine{})
	database.Connection.AutoMigrate(&TrainRecord{})
	database.Connection.AutoMigrate(&RoRoadList{})
	database.Connection.AutoMigrate(&CountryPoint{})
	database.Connection.AutoMigrate(&CountryLocation{})
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_external_api_audit_events_provider_request_time ON external_api_audit_events (provider, request_time)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_external_api_audit_events_request_time ON external_api_audit_events (request_time)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_external_api_audit_events_status_request_time ON external_api_audit_events (status_class, request_time)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_created_at ON marker_creation_outcome_logs (created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_status_created_at ON marker_creation_outcome_logs (status, created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_link_created_at ON marker_creation_outcome_logs (link, created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_marker_id_created_at ON marker_creation_outcome_logs (marker_id, created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_external_run_id_created_at ON marker_creation_outcome_logs (external_run_id, created_at)")

	log.Println("auto migration completed")
}
