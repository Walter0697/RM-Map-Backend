package service

import (
	"encoding/json"
	"fmt"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"strings"
	"testing"
)

func TestSocialMediaAnalysisOutputSchemaRequiresEveryProperty(t *testing.T) {
	schema := socialMediaAnalysisOutputSchema()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected output schema properties")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("expected output schema required fields")
	}
	if len(required) != len(properties) {
		t.Fatalf("strict output schema must require every property: required=%d properties=%d", len(required), len(properties))
	}
	for name := range properties {
		found := false
		for _, item := range required {
			if item == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("property %q is missing from required", name)
		}
	}
	evidence, ok := properties["evidence"].(map[string]any)
	if !ok {
		t.Fatal("expected evidence schema")
	}
	items, ok := evidence["items"].(map[string]any)
	if !ok {
		t.Fatal("expected evidence item schema")
	}
	if _, ok := items["properties"].(map[string]any); !ok {
		t.Fatal("expected evidence item properties")
	}
}

func TestDecodeAgentAnalysisResultAcceptsObjectAndEncodedJSON(t *testing.T) {
	object := json.RawMessage(`{"recommendation":"completed","confidence":0.9}`)
	encoded, err := json.Marshal(string(object))
	if err != nil {
		t.Fatalf("encode result: %v", err)
	}
	for name, raw := range map[string]json.RawMessage{
		"object":  object,
		"encoded": encoded,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := decodeAgentAnalysisResult(raw)
			if err != nil {
				t.Fatalf("decode result: %v", err)
			}
			if result.Recommendation != AgentRecommendationCompleted || result.Confidence != 0.9 {
				t.Fatalf("unexpected result: %#v", result)
			}
		})
	}
}

func TestDecodeAgentAnalysisResultPreservesOptionalImageURL(t *testing.T) {
	result, err := decodeAgentAnalysisResult(json.RawMessage(`{"recommendation":"create_marker","image_url":" https://cdn.example.com/photo.jpg "}`))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.ImageURL != " https://cdn.example.com/photo.jpg " {
		t.Fatalf("unexpected image URL: %q", result.ImageURL)
	}

	withoutImage, err := decodeAgentAnalysisResult(json.RawMessage(`{"recommendation":"completed"}`))
	if err != nil {
		t.Fatalf("decode result without image: %v", err)
	}
	if withoutImage.ImageURL != "" {
		t.Fatalf("expected omitted image URL to remain empty, got %q", withoutImage.ImageURL)
	}
}

func TestBuildSocialMediaAnalysisMarkerInputIncludesOptionalImageURL(t *testing.T) {
	withImage := buildSocialMediaAnalysisMarkerInput(AgentAnalysisResult{ImageURL: " https://cdn.example.com/photo.jpg "})
	if withImage.ImageLink == nil || *withImage.ImageLink != "https://cdn.example.com/photo.jpg" {
		t.Fatalf("expected trimmed image link, got %#v", withImage.ImageLink)
	}

	withoutImage := buildSocialMediaAnalysisMarkerInput(AgentAnalysisResult{})
	if withoutImage.ImageLink != nil {
		t.Fatalf("expected absent image link to remain nil, got %#v", withoutImage.ImageLink)
	}
}

func TestSocialMediaAnalysisPromptAndSchemaDescribeImageURL(t *testing.T) {
	prompt := buildSocialMediaAnalysisPrompt(NormalizedSocialPost{
		SourceURL:   "https://example.com/post",
		FinalURL:    "https://example.com/post",
		Title:       "Weekend Food",
		Description: "A food festival",
		Hashtags:    []string{"food", "hong_kong"},
		ImageURLs:   []string{"https://cdn.example.com/photo.jpg"},
		VideoURLs:   []string{"https://cdn.example.com/video.mp4"},
		RawBody:     "og:image: https://cdn.example.com/photo.jpg",
	})
	for _, value := range []string{"image_url", "og:image", "Weekend Food", "A food festival", "hong_kong", "photo.jpg", "video.mp4"} {
		if !strings.Contains(prompt, value) {
			t.Fatalf("prompt does not include normalized source value %q: %s", value, prompt)
		}
	}
	for _, field := range []string{"tags", "price", "need_booking", "estimate_time", "from_time", "to_time"} {
		if !strings.Contains(prompt, field) {
			t.Fatalf("prompt does not describe %s extraction: %s", field, prompt)
		}
	}

	schema := socialMediaAnalysisOutputSchema()
	properties := schema["properties"].(map[string]any)
	for _, field := range []string{"image_url", "tags", "price", "need_booking", "estimate_time", "from_time", "to_time"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("expected %s in output schema", field)
		}
		found := false
		for _, required := range schema["required"].([]string) {
			if required == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %s to be required by the strict output schema", field)
		}
	}
}

func TestDecodeAgentAnalysisResultPreservesMarkerMetadata(t *testing.T) {
	result, err := decodeAgentAnalysisResult(json.RawMessage(`{"recommendation":"create_marker","tags":["food","#korean"],"price":"middle","need_booking":true,"estimate_time":"medium","from_time":"2026-11-03 10:00:00+00","to_time":null}`))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(result.Tags) != 2 || result.Price != "middle" || !result.NeedBooking || result.EstimateTime != "medium" {
		t.Fatalf("unexpected metadata: %#v", result)
	}
	if result.FromTime == nil || *result.FromTime != "2026-11-03 10:00:00+00" || result.ToTime != nil {
		t.Fatalf("unexpected event times: from=%v to=%v", result.FromTime, result.ToTime)
	}
}

func TestBuildSocialMediaAnalysisMarkerInputMapsMetadataAndHashtags(t *testing.T) {
	fromTime := "2026-11-03 10:00:00+00"
	result := AgentAnalysisResult{
		Description:  "A Korean restaurant",
		Tags:         []string{" food ", "#Korean", "", "street food"},
		Price:        "middle",
		NeedBooking:  true,
		EstimateTime: "medium",
		FromTime:     &fromTime,
	}
	input := buildSocialMediaAnalysisMarkerInput(result)
	if input.Description == nil || *input.Description != "A Korean restaurant\n\n#food #korean #street_food" {
		t.Fatalf("unexpected description: %v", input.Description)
	}
	if input.Price == nil || *input.Price != "middle" || input.NeedBooking == nil || !*input.NeedBooking || input.EstimateTime == nil || *input.EstimateTime != "medium" {
		t.Fatalf("unexpected marker metadata: price=%v booking=%v estimate=%v", input.Price, input.NeedBooking, input.EstimateTime)
	}
	if input.FromTime == nil || *input.FromTime != fromTime || input.ToTime != nil {
		t.Fatalf("unexpected marker event times: from=%v to=%v", input.FromTime, input.ToTime)
	}
}

func TestCreateMarkerWithOptionalImageRetriesWithoutBrokenImage(t *testing.T) {
	imageURL := "https://example.com/broken.png"
	input := buildSocialMediaAnalysisMarkerInput(AgentAnalysisResult{ImageURL: imageURL})
	calls := 0
	marker, err := createMarkerWithOptionalImage(input, func(input model.NewMarker) (*dbmodel.Marker, error) {
		calls++
		if input.ImageLink != nil {
			return nil, fmt.Errorf("invalid image")
		}
		return &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 123}}}, nil
	})
	if err != nil || marker == nil || marker.ID != 123 {
		t.Fatalf("expected fallback marker, marker=%v err=%v", marker, err)
	}
	if calls != 2 {
		t.Fatalf("expected image attempt plus fallback, calls=%d", calls)
	}
}
