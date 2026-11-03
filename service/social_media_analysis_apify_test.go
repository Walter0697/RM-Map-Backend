package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"mapmarker/backend/config"
)

func TestApifyClientRunsActorAndNormalizesDataset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/acts/test~actor/runs" {
			if r.URL.Query().Get("token") != "apify-token" {
				t.Fatalf("expected Apify token")
			}
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatalf("decode actor input: %v", err)
			}
			if input["url"] != "https://www.instagram.com/p/abc" {
				t.Fatalf("expected submitted URL, got %q", input["url"])
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"id":"run-1","defaultDatasetId":"dataset-1","status":"SUCCEEDED"}}`))
			return
		}
		if r.URL.Path == "/v2/datasets/dataset-1/items" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{
				"url":"https://www.instagram.com/p/abc",
				"caption":"Great #Food #food",
				"displayUrl":"https://cdn.example/photo.jpg",
				"videoUrl":"https://cdn.example/video.mp4",
				"title":"Cafe",
				"description":"Dinner"
			}]`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	resolver := NewSecretResolver(config.InfisicalSetting{ApifyToken: "apify-token"}, server.Client())
	client := NewApifyClient(server.URL, "test~actor", resolver, server.Client())
	parsed, _ := url.Parse("https://www.instagram.com/p/abc")
	post, err := client.Fetch(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if post.Title != "Cafe" || post.Description != "Dinner" {
		t.Fatalf("unexpected metadata: %+v", post)
	}
	if len(post.ImageURLs) != 1 || len(post.VideoURLs) != 1 {
		t.Fatalf("expected media URLs, got images=%v videos=%v", post.ImageURLs, post.VideoURLs)
	}
	if strings.Join(post.Hashtags, ",") != "food" {
		t.Fatalf("expected normalized hashtags, got %v", post.Hashtags)
	}
}

func TestSelectConfiguredSourceAdapterUsesApifyForSocialHosts(t *testing.T) {
	resolver := NewSecretResolver(config.InfisicalSetting{ApifyToken: "token"}, http.DefaultClient)
	instagram := SelectConfiguredSourceAdapter("https://www.instagram.com/p/abc", "actor", resolver, http.DefaultClient)
	threads := SelectConfiguredSourceAdapter("https://www.threads.net/@user/post/abc", "actor", resolver, http.DefaultClient)
	generic := SelectConfiguredSourceAdapter("https://example.com/post", "actor", resolver, http.DefaultClient)
	if instagram.Name() != "instagram" || threads.Name() != "threads" || generic.Name() != "generic" {
		t.Fatalf("unexpected adapter names: %s %s %s", instagram.Name(), threads.Name(), generic.Name())
	}
}
