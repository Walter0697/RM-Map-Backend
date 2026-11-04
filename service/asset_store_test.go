package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
)

type recordingAssetStore struct {
	keys []string
}

func (store *recordingAssetStore) Put(_ context.Context, key string, content io.Reader, size int64, contentType string) (StoredAsset, error) {
	body, err := io.ReadAll(content)
	if err != nil {
		return StoredAsset{}, err
	}
	store.keys = append(store.keys, key+"|"+contentType)
	return StoredAsset{Bucket: "test", Key: key, ContentType: contentType, Size: int64(len(body)), SHA256: "checksum", RetentionClass: "test"}, nil
}

type fakeAssetClient struct {
	bucket      string
	key         string
	contentType string
	body        []byte
}

func (client *fakeAssetClient) PutObject(_ context.Context, bucket, key string, content []byte, contentType string) error {
	client.bucket = bucket
	client.key = key
	client.contentType = contentType
	client.body = append([]byte(nil), content...)
	return nil
}

func TestBuildSocialMediaAssetKey(t *testing.T) {
	if got := buildSocialMediaAssetKey("analysis-123", "source/image", 2, "jpg"); got != "social-analysis/analysis-123/source/image-2.jpg" {
		t.Fatalf("unexpected asset key: %q", got)
	}
	if got := buildSocialMediaAssetKey(" analysis/123 ", "scraper", 0, "json"); got != "social-analysis/analysis_123/scraper.json" {
		t.Fatalf("unexpected normalized asset key: %q", got)
	}
}

func TestPrepareSocialAnalysisAssetCalculatesChecksumAndEnforcesLimit(t *testing.T) {
	asset, err := prepareSocialAnalysisAsset(strings.NewReader("hello"), 64, "text/plain")
	if err != nil {
		t.Fatalf("prepare asset: %v", err)
	}
	if asset.Size != 5 || asset.ContentType != "text/plain" || asset.SHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("unexpected prepared asset: %+v", asset)
	}
	if _, err := prepareSocialAnalysisAsset(strings.NewReader("too long"), 3, "text/plain"); err == nil {
		t.Fatal("expected asset size limit error")
	}
}

func TestMinIOAssetStoreUploadsPrivateObjectMetadata(t *testing.T) {
	client := &fakeAssetClient{}
	store, err := newTestMinIOAssetStore(config.MinIOSetting{Enable: true, Bucket: "analysis", MaxBytes: 64}, client)
	if err != nil {
		t.Fatalf("create asset store: %v", err)
	}
	stored, err := store.Put(context.Background(), "social-analysis/job-1/source.json", bytes.NewBufferString("{}"), 2, "application/json")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	if client.bucket != "analysis" || client.key != "social-analysis/job-1/source.json" || client.contentType != "application/json" || string(client.body) != "{}" {
		t.Fatalf("unexpected upload: bucket=%q key=%q content_type=%q body=%q", client.bucket, client.key, client.contentType, client.body)
	}
	if stored.Key != client.key || stored.Bucket != client.bucket || stored.Size != 2 || stored.SHA256 != "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a" {
		t.Fatalf("unexpected stored asset: %+v", stored)
	}
}

func TestMinIOAssetStoreDisabledReturnsNoop(t *testing.T) {
	store, err := NewMinIOAssetStore(config.MinIOSetting{}, nil, nil)
	if err != nil {
		t.Fatalf("create disabled asset store: %v", err)
	}
	if store != nil {
		t.Fatalf("expected disabled store to be nil, got %#v", store)
	}
}

func TestCaptureSocialAnalysisArtifactsStoresBoundedSourceAndMedia(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()
	previousDownloader := downloadSocialAnalysisMediaFn
	downloadSocialAnalysisMediaFn = func(_ context.Context, rawURL string) ([]byte, string, string, error) {
		if rawURL != server.URL+"/photo.jpg" {
			t.Fatalf("unexpected media URL %q", rawURL)
		}
		return []byte("image"), "image/jpeg", "jpeg", nil
	}
	t.Cleanup(func() { downloadSocialAnalysisMediaFn = previousDownloader })

	store := &recordingAssetStore{}
	job := &dbmodel.SocialMediaAnalysisJob{JobID: "analysis-1"}
	captureSocialAnalysisArtifacts(context.Background(), job, NormalizedSocialPost{
		RawBody:   "<html>source</html>",
		ImageURLs: []string{server.URL + "/photo.jpg"},
	}, []byte(`{"source_url":"https://example.com"}`), store)
	if len(store.keys) != 3 {
		t.Fatalf("expected source JSON, raw body, and image artifacts, got %v", store.keys)
	}
	for _, key := range []string{"source.json", "source/raw-body.html", "image-1.jpeg"} {
		found := false
		for _, stored := range store.keys {
			if strings.Contains(stored, key) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected artifact key containing %q, got %v", key, store.keys)
		}
	}
}
