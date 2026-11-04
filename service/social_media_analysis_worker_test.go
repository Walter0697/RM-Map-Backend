package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"reflect"
	"strings"
	"testing"
)

type fakeSocialMediaRunnerClient struct {
	submitted []RunnerSubmitRequest
}

func (client *fakeSocialMediaRunnerClient) Submit(_ context.Context, request RunnerSubmitRequest) (RunnerJob, error) {
	client.submitted = append(client.submitted, request)
	return RunnerJob{ID: "comparison-runner-1", Status: RunnerJobStatusAccepted}, nil
}

func (client *fakeSocialMediaRunnerClient) Get(context.Context, string) (RunnerJob, error) {
	return RunnerJob{}, fmt.Errorf("not implemented")
}

type fakeGooglePlacesProvider struct {
	places []GooglePlaceCandidate
	query  string
}

func (provider *fakeGooglePlacesProvider) Search(_ context.Context, query string) ([]GooglePlaceCandidate, error) {
	provider.query = query
	return provider.places, nil
}

func TestMaybeSubmitLocationComparisonKeepsVerifiedGoogleLocation(t *testing.T) {
	provider := &fakeGooglePlacesProvider{places: []GooglePlaceCandidate{{PlaceID: "place-1", Name: "IFC Mall", Address: "8 Finance Street, Central", Latitude: 22.2855, Longitude: 114.1579}}}
	previousProvider := googlePlacesProviderFn
	googlePlacesProviderFn = func() GooglePlacesProvider { return provider }
	t.Cleanup(func() { googlePlacesProviderFn = previousProvider })

	result := AgentAnalysisResult{Recommendation: AgentRecommendationCreateMarker, Location: &LocationCandidate{Name: "IFC Mall", Address: "8 Finance Street, Central", Latitude: 22.2854, Longitude: 114.1579}}
	runner, submitted, err := maybeSubmitLocationComparison(context.Background(), &dbmodel.SocialMediaAnalysisJob{}, &result)
	if err != nil || submitted || runner.ID != "" {
		t.Fatalf("expected verified location to avoid Agent 2, runner=%#v submitted=%v err=%v", runner, submitted, err)
	}
	if result.Location.Name != "IFC Mall" || result.Location.Latitude != 22.2855 {
		t.Fatalf("expected Google location to replace agent location, got %#v", result.Location)
	}
	if provider.query != "IFC Mall, 8 Finance Street, Central" {
		t.Fatalf("unexpected Google query %q", provider.query)
	}
}

func TestMaybeSubmitLocationComparisonSubmitsAgentTwoForAmbiguousCandidates(t *testing.T) {
	provider := &fakeGooglePlacesProvider{places: []GooglePlaceCandidate{
		{PlaceID: "place-1", Name: "Cafe", Address: "Central", Latitude: 22.28, Longitude: 114.15},
		{PlaceID: "place-2", Name: "Cafe", Address: "Central", Latitude: 22.29, Longitude: 114.16},
	}}
	runner := &fakeSocialMediaRunnerClient{}
	previousProvider := googlePlacesProviderFn
	previousRunner := socialMediaRunnerClientFn
	googlePlacesProviderFn = func() GooglePlacesProvider { return provider }
	socialMediaRunnerClientFn = func() socialMediaRunnerClient { return runner }
	t.Cleanup(func() {
		googlePlacesProviderFn = previousProvider
		socialMediaRunnerClientFn = previousRunner
	})

	result := AgentAnalysisResult{Recommendation: AgentRecommendationCreateMarker, Location: &LocationCandidate{Name: "Cafe", Address: "Central", Latitude: 22.285, Longitude: 114.155}}
	job := &dbmodel.SocialMediaAnalysisJob{SourceJSON: `{"source_url":"https://example.com/post","title":"Cafe post","description":"A cafe"}`}
	comparison, submitted, err := maybeSubmitLocationComparison(context.Background(), job, &result)
	if err != nil || !submitted || comparison.ID != "comparison-runner-1" {
		t.Fatalf("expected Agent 2 submission, runner=%#v submitted=%v err=%v", comparison, submitted, err)
	}
	if len(runner.submitted) != 1 || runner.submitted[0].Operation != "social-media-location-comparison" {
		t.Fatalf("unexpected Agent 2 submissions: %#v", runner.submitted)
	}
	for _, expected := range []string{"Cafe post", "First agent result", "Google Places candidates", "needs_input"} {
		if !strings.Contains(runner.submitted[0].Prompt, expected) {
			t.Fatalf("comparison prompt missing %q: %s", expected, runner.submitted[0].Prompt)
		}
	}
}

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

func TestBuildSocialMediaAnalysisPromptIncludesAllowedMarkerTypes(t *testing.T) {
	prompt := buildSocialMediaAnalysisPrompt(NormalizedSocialPost{}, []string{"restaurant", "event"})
	if !strings.Contains(prompt, "Allowed marker types: restaurant, event") {
		t.Fatalf("expected allowed marker types in prompt, got %q", prompt)
	}
}

func TestSocialMediaAnalysisOutputSchemaConstrainsMarkerType(t *testing.T) {
	schema := socialMediaAnalysisOutputSchema([]string{"restaurant", "event"})
	properties := schema["properties"].(map[string]any)
	markerType := properties["marker_type"].(map[string]any)
	enum := markerType["enum"].([]string)
	if !reflect.DeepEqual(enum, []string{"restaurant", "event"}) {
		t.Fatalf("expected marker type enum, got %#v", enum)
	}
}

func TestResolveSocialMediaAnalysisMarkerTypeFallsBackToRestaurant(t *testing.T) {
	allowed := []string{"restaurant", "event"}
	if got := resolveSocialMediaAnalysisMarkerType("food", allowed); got != "restaurant" {
		t.Fatalf("expected unsupported marker type to fall back to restaurant, got %q", got)
	}
	if got := resolveSocialMediaAnalysisMarkerType("event", allowed); got != "event" {
		t.Fatalf("expected supported marker type to be preserved, got %q", got)
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

func TestImageCandidatesPreferScrapedSourceAndFallbackToAgent(t *testing.T) {
	job := &dbmodel.SocialMediaAnalysisJob{SourceJSON: `{"image_urls":["https://cdn.example.com/scraped.jpg"]}`}
	result := AgentAnalysisResult{ImageURL: "https://cdn.example.com/agent.jpg"}
	candidates := imageCandidatesForSocialMediaMarker(job, result)
	if len(candidates) != 2 || candidates[0] != "https://cdn.example.com/scraped.jpg" || candidates[1] != "https://cdn.example.com/agent.jpg" {
		t.Fatalf("unexpected image candidates: %v", candidates)
	}

	calls := make([]string, 0, 3)
	input := model.NewMarker{}
	marker, err := createMarkerWithImageCandidates(input, candidates, func(input model.NewMarker) (*dbmodel.Marker, error) {
		if input.ImageLink == nil {
			calls = append(calls, "none")
			return &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 321}}}, nil
		}
		calls = append(calls, *input.ImageLink)
		if strings.HasSuffix(*input.ImageLink, "scraped.jpg") {
			return nil, fmt.Errorf("scraped image unavailable")
		}
		return &dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 322}}}, nil
	})
	if err != nil || marker.ID != 322 {
		t.Fatalf("expected agent image fallback marker, got marker=%#v err=%v", marker, err)
	}
	if strings.Join(calls, ",") != "https://cdn.example.com/scraped.jpg,https://cdn.example.com/agent.jpg" {
		t.Fatalf("unexpected create attempts: %v", calls)
	}
}
