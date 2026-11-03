package service

import "encoding/json"

type SocialMediaAnalysisRequest struct {
	URL string `json:"url"`
}

type SocialMediaAnalysisResponse struct {
	JobID       string          `json:"job_id"`
	Status      string          `json:"status"`
	Stage       string          `json:"stage,omitempty"`
	RunnerJobID string          `json:"-"`
	Prompt      string          `json:"-"`
	Result      json.RawMessage `json:"result,omitempty"`
}

type NormalizedSocialPost struct {
	SourceURL string `json:"source_url"`
	FinalURL  string `json:"final_url"`
	RawBody   string `json:"-"`
}
