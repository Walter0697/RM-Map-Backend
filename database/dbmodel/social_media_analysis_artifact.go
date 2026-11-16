package dbmodel

const (
	SocialMediaAnalysisArtifactKindSourceJSON = "source_json"
	SocialMediaAnalysisArtifactKindImage      = "image"
	SocialMediaAnalysisArtifactKindVideo      = "video"
)

type SocialMediaAnalysisArtifact struct {
	BaseModel
	AnalysisJobID  uint   `json:"analysis_job_id" gorm:"not null;index"`
	JobID          string `json:"job_id" gorm:"size:64;not null;index"`
	ObjectKey      string `json:"object_key" gorm:"type:text;not null"`
	Kind           string `json:"kind" gorm:"size:32;not null"`
	Provider       string `json:"provider,omitempty" gorm:"size:64"`
	ContentType    string `json:"content_type" gorm:"size:128;not null"`
	SizeBytes      int64  `json:"size_bytes" gorm:"not null"`
	SHA256         string `json:"sha256" gorm:"size:64;not null"`
	RetentionClass string `json:"retention_class" gorm:"size:64;not null"`
}
