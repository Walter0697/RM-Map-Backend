package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mapmarker/backend/config"
)

type recordingObservabilitySpan struct {
	name       string
	attributes map[string]string
	ok         bool
}

func (span *recordingObservabilitySpan) SetAttribute(key, value string) {
	if span.attributes == nil {
		span.attributes = make(map[string]string)
	}
	span.attributes[key] = value
}
func (span *recordingObservabilitySpan) AddEvent(string)             {}
func (span *recordingObservabilitySpan) RecordError(error)           { span.ok = false }
func (span *recordingObservabilitySpan) SetStatus(ok bool, _ string) { span.ok = ok }
func (span *recordingObservabilitySpan) End()                        {}

type recordingObservabilityTracer struct {
	spans []*recordingObservabilitySpan
}

func (tracer *recordingObservabilityTracer) Start(ctx context.Context, name string, attributes map[string]string) (context.Context, SpanHandle) {
	span := &recordingObservabilitySpan{name: name, attributes: attributes, ok: true}
	tracer.spans = append(tracer.spans, span)
	return ctx, span
}

type googlePlacesTestSecrets map[string]string

func (s googlePlacesTestSecrets) Resolve(_ context.Context, name string) (string, error) {
	return s[name], nil
}

func TestGooglePlacesSearchBuildsRequestAndParsesCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/places:searchText" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Goog-Api-Key") != "google-key" {
			t.Fatalf("unexpected API key header: %q", r.Header.Get("X-Goog-Api-Key"))
		}
		if r.Header.Get("X-Goog-FieldMask") != googlePlacesFieldMask {
			t.Fatalf("unexpected field mask: %q", r.Header.Get("X-Goog-FieldMask"))
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		if !strings.Contains(string(body), `"textQuery":"IFC Hong Kong"`) {
			t.Fatalf("request did not contain text query: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"places":[{"id":"place-1","displayName":{"text":"IFC Mall"},"formattedAddress":"8 Finance Street, Central, Hong Kong","location":{"latitude":22.2854,"longitude":114.1579},"types":["shopping_mall"],"googleMapsUri":"https://maps.google.com/?cid=1"}]}`))
	}))
	defer server.Close()

	provider := NewGooglePlacesProvider(config.GoogleMapsSetting{BaseURL: server.URL, TimeoutMS: 1000, MaxResults: 5}, googlePlacesTestSecrets{"GOOGLE_MAPS_API_KEY": "google-key"}, server.Client())
	places, err := provider.Search(context.Background(), "IFC Hong Kong")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(places) != 1 || places[0].PlaceID != "place-1" || places[0].Name != "IFC Mall" || places[0].Latitude != 22.2854 {
		t.Fatalf("unexpected places: %#v", places)
	}
}

func TestGooglePlacesSearchRejectsProviderErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"status":"PERMISSION_DENIED","message":"API key not valid"}}`))
	}))
	defer server.Close()

	provider := NewGooglePlacesProvider(config.GoogleMapsSetting{BaseURL: server.URL, TimeoutMS: 1000, MaxResults: 5}, googlePlacesTestSecrets{"GOOGLE_MAPS_API_KEY": "google-key"}, server.Client())
	_, err := provider.Search(context.Background(), "IFC Hong Kong")
	if err == nil || !strings.Contains(err.Error(), "API key not valid") {
		t.Fatal("expected provider error")
	}
}

func TestGooglePlacesSearchRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{"))
	}))
	defer server.Close()

	provider := NewGooglePlacesProvider(config.GoogleMapsSetting{BaseURL: server.URL, TimeoutMS: 1000, MaxResults: 5}, googlePlacesTestSecrets{"GOOGLE_MAPS_API_KEY": "google-key"}, server.Client())
	if _, err := provider.Search(context.Background(), "IFC Hong Kong"); err == nil {
		t.Fatal("expected malformed response error")
	}
}

func TestGooglePlacesSearchRecordsProviderSpanWithoutRawQueryOrKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"places":[]}`))
	}))
	defer server.Close()

	tracer := &recordingObservabilityTracer{}
	previousTracer := socialAnalysisTracerFn
	socialAnalysisTracerFn = func() socialAnalysisTracerProvider { return tracer }
	t.Cleanup(func() { socialAnalysisTracerFn = previousTracer })

	provider := NewGooglePlacesProvider(config.GoogleMapsSetting{BaseURL: server.URL, TimeoutMS: 1000, MaxResults: 5}, googlePlacesTestSecrets{"GOOGLE_MAPS_API_KEY": "google-key"}, server.Client())
	if _, err := provider.Search(context.Background(), "IFC Hong Kong"); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(tracer.spans) != 1 {
		t.Fatalf("expected one provider span, got %d", len(tracer.spans))
	}
	span := tracer.spans[0]
	if span.name != "google_places.search" || span.attributes["provider.name"] != "google_places" || span.attributes["provider.operation"] != "search" || span.attributes["result.candidate_count"] != "0" || !span.ok {
		t.Fatalf("unexpected Google span: %#v", span)
	}
	if span.attributes["provider.query"] != "" || span.attributes["google.api_key"] != "" {
		t.Fatalf("Google span exposed sensitive query/key metadata: %#v", span.attributes)
	}
	if span.attributes["provider.query_sha256"] == "" {
		t.Fatalf("expected query fingerprint: %#v", span.attributes)
	}
}
