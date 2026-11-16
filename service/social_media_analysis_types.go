package service

import (
	"encoding/json"
	"fmt"
)

type SocialMediaAnalysisRequest struct {
	URL string `json:"url"`
}

type SocialMediaAnalysisResponse struct {
	JobID       string          `json:"job_id"`
	Status      string          `json:"status"`
	Stage       string          `json:"stage,omitempty"`
	MarkerID    *uint           `json:"marker_id,omitempty"`
	RunnerJobID string          `json:"-"`
	Prompt      string          `json:"-"`
	Result      json.RawMessage `json:"result,omitempty"`
}

type NormalizedSocialPost struct {
	SourceURL   string            `json:"source_url"`
	FinalURL    string            `json:"final_url"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Hashtags    []string          `json:"hashtags,omitempty"`
	ImageURLs   []string          `json:"image_urls,omitempty"`
	VideoURLs   []string          `json:"video_urls,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RawBody     string            `json:"-"`
}

const (
	MediaTypeImage = "image"
	MediaTypeVideo = "video"
)

type MediaReference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type ImageAnalysisInput struct {
	Media MediaReference
}

type VideoInput struct {
	Media MediaReference
}

type PreparedVideo struct {
	FrameURLs  []string `json:"frame_urls"`
	Transcript string   `json:"transcript,omitempty"`
}

type MediaAnalysisResult struct {
	Facts      []string `json:"facts"`
	Confidence float64  `json:"confidence"`
}

const (
	AgentRecommendationCompleted            = "completed"
	AgentRecommendationNeedsInput           = "needs_input"
	AgentRecommendationAwaitingConfirmation = "awaiting_confirmation"
	AgentRecommendationCreateMarker         = "create_marker"
)

type AgentAnalysisInput struct {
	Post       NormalizedSocialPost `json:"post"`
	Media      MediaAnalysisResult  `json:"media"`
	Candidates []MarkerCandidate    `json:"candidates"`
}

type AgentAnalysisResult struct {
	Recommendation string             `json:"recommendation"`
	Description    string             `json:"description,omitempty"`
	ImageURL       string             `json:"image_url,omitempty"`
	MarkerType     string             `json:"marker_type,omitempty"`
	Tags           []string           `json:"tags,omitempty"`
	Price          string             `json:"price,omitempty"`
	NeedBooking    bool               `json:"need_booking"`
	EstimateTime   string             `json:"estimate_time,omitempty"`
	FromTime       *string            `json:"from_time,omitempty"`
	ToTime         *string            `json:"to_time,omitempty"`
	Location       *LocationCandidate `json:"location,omitempty"`
	Evidence       []EvidenceItem     `json:"evidence,omitempty"`
	Confidence     float64            `json:"confidence"`
	FollowUp       string             `json:"follow_up,omitempty"`
}

type LocationCandidate struct {
	Name       string  `json:"name,omitempty"`
	Address    string  `json:"address,omitempty"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	Confidence float64 `json:"confidence"`
}

type EvidenceItem struct {
	Source string `json:"source"`
	Claim  string `json:"claim"`
}

type MarkerCandidate struct {
	ID        uint    `json:"id"`
	Name      string  `json:"name,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	DistanceM float64 `json:"distance_m,omitempty"`
}

func ValidateAgentAnalysisResult(result AgentAnalysisResult) error {
	switch result.Recommendation {
	case AgentRecommendationCompleted, AgentRecommendationNeedsInput, AgentRecommendationAwaitingConfirmation, AgentRecommendationCreateMarker:
	default:
		return fmt.Errorf("unknown agent recommendation %q", result.Recommendation)
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		return fmt.Errorf("agent confidence must be between 0 and 1")
	}
	return nil
}
