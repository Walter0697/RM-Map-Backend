package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"mapmarker/backend/config"
)

func TestValidatePublicSourceURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "public https", input: "https://example.com/post"},
		{name: "public http", input: "http://example.com/post"},
		{name: "missing scheme", input: "example.com/post", wantErr: true},
		{name: "unsupported scheme", input: "file:///tmp/post.html", wantErr: true},
		{name: "embedded credentials", input: "https://user:pass@example.com/post", wantErr: true},
		{name: "localhost", input: "http://localhost/post", wantErr: true},
		{name: "loopback", input: "http://127.0.0.1/post", wantErr: true},
		{name: "private network", input: "http://192.168.1.10/post", wantErr: true},
		{name: "link local", input: "http://169.254.169.254/latest/meta-data", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ValidatePublicSourceURL(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected validation error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if parsed.String() != test.input {
				t.Fatalf("expected normalized URL %q, got %q", test.input, parsed.String())
			}
		})
	}
}

func TestSelectSourceAdapter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{name: "instagram", input: "https://www.instagram.com/p/abc123", wantName: "instagram"},
		{name: "threads", input: "https://www.threads.net/@someone/post/abc123", wantName: "threads"},
		{name: "generic", input: "https://example.com/post", wantName: "generic"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := SelectSourceAdapter(test.input)
			if adapter == nil {
				t.Fatal("expected source adapter")
			}
			if got := adapter.Name(); got != test.wantName {
				t.Fatalf("expected adapter %q, got %q", test.wantName, got)
			}
		})
	}
}

func TestSelectSocialMediaAnalysisSourceAdapterUsesConfiguredProvider(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()
	config.Data.Infisical.ApifyActorID = "actor"
	config.Data.Infisical.ApifyToken = "token"
	adapter := selectSocialMediaAnalysisSourceAdapter("https://www.instagram.com/p/abc")
	if adapter.Name() != "instagram-guest" {
		t.Fatalf("expected configured Instagram adapter, got %q", adapter.Name())
	}
}

func TestSelectConfiguredSourceAdapterUsesGuestForInstagramAndApifyForThreads(t *testing.T) {
	resolver := NewSecretResolver(config.InfisicalSetting{ApifyToken: "token"}, http.DefaultClient)
	instagram := SelectConfiguredSourceAdapter("https://www.instagram.com/p/abc", "actor", resolver, http.DefaultClient)
	threads := SelectConfiguredSourceAdapter("https://www.threads.net/@user/post/abc", "actor", resolver, http.DefaultClient)
	if instagram.Name() != "instagram-guest" {
		t.Fatalf("expected Instagram Guest adapter, got %q", instagram.Name())
	}
	if threads.Name() != "threads" {
		t.Fatalf("expected Threads Apify adapter, got %q", threads.Name())
	}
}

func TestFetchGenericSourcePreservesFinalURLAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head>
			<title>Weekend Food</title>
			<meta name="description" content="A food festival">
			<meta property="og:image" content="/images/hero.jpg">
			<meta property="og:video" content="/videos/post.mp4">
		</head><body>post content #Food #hong_kong #food</body></html>`))
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL + "/post")
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	result, err := fetchGenericSource(context.Background(), parsed, server.Client(), 1024)
	if err != nil {
		t.Fatalf("fetch generic source: %v", err)
	}
	if result.FinalURL != parsed.String() {
		t.Fatalf("expected final URL %q, got %q", parsed.String(), result.FinalURL)
	}
	if !strings.Contains(result.RawBody, "post content") {
		t.Fatalf("expected fetched body to be retained, got %q", result.RawBody)
	}
	if result.Title != "Weekend Food" || result.Description != "A food festival" {
		t.Fatalf("expected title and description, got %q and %q", result.Title, result.Description)
	}
	if len(result.ImageURLs) != 1 || !strings.HasSuffix(result.ImageURLs[0], "/images/hero.jpg") {
		t.Fatalf("expected normalized image URL, got %v", result.ImageURLs)
	}
	if len(result.VideoURLs) != 1 || !strings.HasSuffix(result.VideoURLs[0], "/videos/post.mp4") {
		t.Fatalf("expected normalized video URL, got %v", result.VideoURLs)
	}
	if strings.Join(result.Hashtags, ",") != "food,hong_kong" {
		t.Fatalf("expected deduplicated hashtags, got %v", result.Hashtags)
	}
}

func TestFetchGenericSourceRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this response is larger than the configured limit"))
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL + "/post")
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	_, err = fetchGenericSource(context.Background(), parsed, server.Client(), 8)
	if err == nil {
		t.Fatal("expected oversized response error")
	}
	if !strings.Contains(err.Error(), "response too large") {
		t.Fatalf("expected response-size error, got %v", err)
	}
}
