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
	database.Connection.AutoMigrate(&ServiceAccount{})
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
	database.Connection.AutoMigrate(&PinGroup{})
	database.Connection.AutoMigrate(&PinGroupAssignment{})
	database.Connection.AutoMigrate(&TypePin{})
	database.Connection.AutoMigrate(&DefaultValue{})
	database.Connection.AutoMigrate(&SystemSetting{})
	database.Connection.AutoMigrate(&Schedule{})
	database.Connection.AutoMigrate(&ScheduleReminderDispatchLog{})
	database.Connection.AutoMigrate(&CalendarProviderConnection{})
	database.Connection.AutoMigrate(&ScheduleCalendarSyncLink{})
	database.Connection.AutoMigrate(&CalendarSyncJob{})
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
	database.Connection.AutoMigrate(&TravelPlan{})
	database.Connection.AutoMigrate(&TravelPlanDailyPlan{})
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_external_api_audit_events_provider_request_time ON external_api_audit_events (provider, request_time)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_external_api_audit_events_request_time ON external_api_audit_events (request_time)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_external_api_audit_events_status_request_time ON external_api_audit_events (status_class, request_time)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_created_at ON marker_creation_outcome_logs (created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_status_created_at ON marker_creation_outcome_logs (status, created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_link_created_at ON marker_creation_outcome_logs (link, created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_marker_id_created_at ON marker_creation_outcome_logs (marker_id, created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_marker_creation_outcome_logs_external_run_id_created_at ON marker_creation_outcome_logs (external_run_id, created_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_schedule_calendar_sync_links_external_event_id ON schedule_calendar_sync_links (external_event_id)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_schedule_calendar_sync_links_status_next_retry ON schedule_calendar_sync_links (sync_status, next_retry_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_calendar_sync_jobs_due ON calendar_sync_jobs (status, next_attempt_at)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_calendar_sync_jobs_schedule_provider ON calendar_sync_jobs (schedule_id, provider_key)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_api_keys_testing ON api_keys (testing)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_markers_testing ON markers (testing)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_schedules_testing ON schedules (testing)")
	database.Connection.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_pin_group_assignments_unique_pin_group ON pin_group_assignments (pin_id, pin_group_id)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_pin_group_assignments_pin_id ON pin_group_assignments (pin_id)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_pin_group_assignments_group_id ON pin_group_assignments (pin_group_id)")
	database.Connection.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_schedule_reminder_dispatch_logs_unique_user_schedule_date ON schedule_reminder_dispatch_logs (user_id, schedule_id, local_date)")
	database.Connection.Exec("CREATE INDEX IF NOT EXISTS idx_schedule_reminder_dispatch_logs_relation_date ON schedule_reminder_dispatch_logs (relation_id, local_date)")

	log.Println("auto migration completed")
}
