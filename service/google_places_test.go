package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mapmarker/backend/config"
)

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
		http.Error(w, "provider failed", http.StatusBadGateway)
	}))
	defer server.Close()

	provider := NewGooglePlacesProvider(config.GoogleMapsSetting{BaseURL: server.URL, TimeoutMS: 1000, MaxResults: 5}, googlePlacesTestSecrets{"GOOGLE_MAPS_API_KEY": "google-key"}, server.Client())
	if _, err := provider.Search(context.Background(), "IFC Hong Kong"); err == nil {
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
