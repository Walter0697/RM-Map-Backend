package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"mapmarker/backend/config"
)

type mapSecretResolver map[string]string

func (resolver mapSecretResolver) Resolve(_ context.Context, name string) (string, error) {
	value := resolver[name]
	if value == "" {
		return "", fmt.Errorf("secret %s is not configured", name)
	}
	return value, nil
}

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

func TestApifyClientResolvesActorIDWhenNotInToml(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/acts/test~actor/runs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"run-1","defaultDatasetId":"dataset-1","status":"SUCCEEDED"}}`))
	}))
	defer server.Close()

	resolver := NewSecretResolver(config.InfisicalSetting{ApifyToken: "token", ApifyActorID: "test~actor"}, server.Client())
	client := NewApifyClient(server.URL, "", resolver, server.Client())
	parsed, _ := url.Parse("https://www.instagram.com/p/abc")
	if _, err := client.Fetch(context.Background(), parsed); err == nil || !strings.Contains(err.Error(), "fetch Apify dataset") {
		t.Fatalf("expected dataset fetch error after actor resolution, got %v", err)
	}
}

func TestApifyClientUsesInstagramActorSecretAndDirectURLInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/acts/apify~instagram-scraper/runs" {
			http.NotFound(w, r)
			return
		}
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode actor input: %v", err)
		}
		directURLs, ok := input["directUrls"].([]any)
		if !ok || len(directURLs) != 1 || directURLs[0] != "https://www.instagram.com/reel/abc" {
			t.Fatalf("expected Instagram directUrls input, got %#v", input)
		}
		if input["resultsType"] != "posts" || input["resultsLimit"] != float64(1) {
			t.Fatalf("expected bounded post input, got %#v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"run-1","defaultDatasetId":"dataset-1","status":"SUCCEEDED"}}`))
	}))
	defer server.Close()

	client := newApifyClient(server.URL, "", "APIFY_INSTAGRAM_ACTOR_ID", "instagram", mapSecretResolver{
		"APIFY_TOKEN":              "apify-token",
		"APIFY_INSTAGRAM_ACTOR_ID": "apify~instagram-scraper",
	}, server.Client())
	parsed, _ := url.Parse("https://www.instagram.com/reel/abc")
	if _, err := client.Fetch(context.Background(), parsed); err == nil || !strings.Contains(err.Error(), "fetch Apify dataset") {
		t.Fatalf("expected dataset fetch error after validating actor input, got %v", err)
	}
}

func TestApifyClientUsesThreadsActorSecretAndPostURLsInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/acts/threads-actor/runs" {
			http.NotFound(w, r)
			return
		}
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode actor input: %v", err)
		}
		postURLs, ok := input["post_urls"].([]any)
		if !ok || len(postURLs) != 1 || postURLs[0] != "https://www.threads.net/@user/post/abc" {
			t.Fatalf("expected Threads post_urls input, got %#v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"run-1","defaultDatasetId":"dataset-1","status":"SUCCEEDED"}}`))
	}))
	defer server.Close()

	client := newApifyClient(server.URL, "", "APIFY_THREADS_ACTOR_ID", "threads", mapSecretResolver{
		"APIFY_TOKEN":            "apify-token",
		"APIFY_THREADS_ACTOR_ID": "threads-actor",
	}, server.Client())
	parsed, _ := url.Parse("https://www.threads.net/@user/post/abc")
	if _, err := client.Fetch(context.Background(), parsed); err == nil || !strings.Contains(err.Error(), "fetch Apify dataset") {
		t.Fatalf("expected dataset fetch error after validating actor input, got %v", err)
	}
}

func TestApifyClientWaitsForActorCompletionBeforeReadingDataset(t *testing.T) {
	var datasetReads atomic.Int32
	var statusReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/acts/test~actor/runs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"id":"run-1","defaultDatasetId":"dataset-1","status":"RUNNING"}}`))
		case "/v2/actor-runs/run-1":
			statusReads.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"status":"SUCCEEDED","defaultDatasetId":"dataset-1"}}`))
		case "/v2/datasets/dataset-1/items":
			datasetReads.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"caption":"Ready #food","displayUrl":"https://cdn.example/photo.jpg"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newApifyClient(server.URL, "test~actor", "APIFY_ACTOR_ID", "legacy", mapSecretResolver{"APIFY_TOKEN": "token"}, server.Client())
	parsed, _ := url.Parse("https://www.instagram.com/p/abc")
	post, err := client.Fetch(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if post.Description != "Ready #food" || datasetReads.Load() != 1 || statusReads.Load() != 1 {
		t.Fatalf("expected completed dataset result, got post=%+v dataset_reads=%d status_reads=%d", post, datasetReads.Load(), statusReads.Load())
	}
}

func TestSelectConfiguredSourceAdapterUsesApifyForThreads(t *testing.T) {
	resolver := NewSecretResolver(config.InfisicalSetting{ApifyToken: "token"}, http.DefaultClient)
	instagram := SelectConfiguredSourceAdapter("https://www.instagram.com/p/abc", "actor", resolver, http.DefaultClient)
	threads := SelectConfiguredSourceAdapter("https://www.threads.net/@user/post/abc", "actor", resolver, http.DefaultClient)
	generic := SelectConfiguredSourceAdapter("https://example.com/post", "actor", resolver, http.DefaultClient)
	if instagram.Name() != "instagram" || threads.Name() != "threads" || generic.Name() != "generic" {
		t.Fatalf("unexpected adapter names: %s %s %s", instagram.Name(), threads.Name(), generic.Name())
	}
}
