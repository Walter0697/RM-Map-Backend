package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestInstagramGuestClientFetchesCarousel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("User-Agent"), "Mozilla") {
				t.Fatalf("expected browser-like homepage request, got %s %q", r.Method, r.Header.Get("User-Agent"))
			}
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-value", Path: "/"})
			w.WriteHeader(http.StatusOK)
		case "/graphql/query":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("X-CSRFToken") != "csrf-value" {
				t.Fatalf("expected CSRF header, got %q", r.Header.Get("X-CSRFToken"))
			}
			if !strings.Contains(r.Header.Get("Cookie"), "csrftoken=csrf-value") {
				t.Fatalf("expected CSRF cookie, got %q", r.Header.Get("Cookie"))
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if r.Form.Get("doc_id") != "test-doc-id" {
				t.Fatalf("expected doc ID, got %q", r.Form.Get("doc_id"))
			}
			if !strings.Contains(r.Form.Get("variables"), `"shortcode":"ABC_123"`) {
				t.Fatalf("expected shortcode in variables, got %q", r.Form.Get("variables"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"xdt_shortcode_media":{"__typename":"XDTGraphSidecar","edge_sidecar_to_children":{"edges":[{"node":{"is_video":false,"dimensions":{"width":100,"height":100},"display_url":"https://cdn.example/image-1.jpg"}},{"node":{"is_video":false,"dimensions":{"width":200,"height":200},"display_url":"https://cdn.example/image-2.jpg"}}]},"edge_media_to_caption":{"edges":[{"node":{"text":"Great place #Food #food"}}]},"owner":{"username":"demo","full_name":"Demo User","is_verified":true},"edge_media_preview_like":{"count":42},"is_ad":false}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewInstagramGuestClient(server.URL, "test-doc-id", server.Client())
	parsed, _ := url.Parse("https://www.instagram.com/reel/ABC_123/?utm_source=test")
	post, err := client.Fetch(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if post.Description != "Great place #Food #food" {
		t.Fatalf("expected caption description, got %q", post.Description)
	}
	if len(post.ImageURLs) != 2 || post.ImageURLs[0] != "https://cdn.example/image-1.jpg" {
		t.Fatalf("expected carousel images, got %v", post.ImageURLs)
	}
	if len(post.VideoURLs) != 0 {
		t.Fatalf("expected no videos, got %v", post.VideoURLs)
	}
	if strings.Join(post.Hashtags, ",") != "food" {
		t.Fatalf("expected normalized hashtags, got %v", post.Hashtags)
	}
	if post.Metadata["owner_username"] != "demo" || post.Metadata["likes_count"] != "42" {
		t.Fatalf("expected owner metadata, got %v", post.Metadata)
	}
}

func TestInstagramGuestClientFetchesVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-value"})
			return
		}
		if r.URL.Path == "/graphql/query" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"xdt_shortcode_media":{"__typename":"XDTGraphVideo","is_video":true,"video_url":"https://cdn.example/video.mp4","display_url":"https://cdn.example/thumb.jpg","edge_media_to_caption":{"edges":[]}}}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewInstagramGuestClient(server.URL, "test-doc-id", server.Client())
	parsed, _ := url.Parse("https://www.instagram.com/p/VIDEO123")
	post, err := client.Fetch(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(post.VideoURLs) != 1 || post.VideoURLs[0] != "https://cdn.example/video.mp4" {
		t.Fatalf("expected video URL, got %v", post.VideoURLs)
	}
	if len(post.ImageURLs) != 1 || post.ImageURLs[0] != "https://cdn.example/thumb.jpg" {
		t.Fatalf("expected thumbnail image, got %v", post.ImageURLs)
	}
}

func TestInstagramGuestClientRejectsMissingMedia(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-value"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()

	client := NewInstagramGuestClient(server.URL, "test-doc-id", server.Client())
	parsed, _ := url.Parse("https://www.instagram.com/p/PRIVATE123")
	if _, err := client.Fetch(context.Background(), parsed); err == nil || !strings.Contains(err.Error(), "no media data") {
		t.Fatalf("expected missing media error, got %v", err)
	}
}

func TestInstagramGuestClientRejectsUnsupportedPath(t *testing.T) {
	client := NewInstagramGuestClient("https://www.instagram.com", "test-doc-id", http.DefaultClient)
	parsed, _ := url.Parse("https://www.instagram.com/explore/")
	if _, err := client.Fetch(context.Background(), parsed); err == nil || !strings.Contains(err.Error(), "shortcode") {
		t.Fatalf("expected shortcode error, got %v", err)
	}
}
