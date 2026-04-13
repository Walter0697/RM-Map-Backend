package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"mapmarker/backend/config"
)

type staticPreviewRoundTripper func(*http.Request) (*http.Response, error)

func (f staticPreviewRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestFetchTomTomStaticMapImageRetriesTransientFailures(t *testing.T) {
	originalFactory := staticPreviewHTTPClientFactory
	originalSleep := staticPreviewSleepFn
	originalTomTomStaticImage := config.Data.TomTomStaticImage
	originalTomTomMapKey := config.Data.APIKEY.TomTomMap
	originalTomTomStaticImageKey := config.Data.APIKEY.TomTomStaticImage
	defer func() {
		staticPreviewHTTPClientFactory = originalFactory
		staticPreviewSleepFn = originalSleep
		config.Data.TomTomStaticImage = originalTomTomStaticImage
		config.Data.APIKEY.TomTomMap = originalTomTomMapKey
		config.Data.APIKEY.TomTomStaticImage = originalTomTomStaticImageKey
	}()

	config.Data.APIKEY.TomTomMap = "map-key"
	config.Data.APIKEY.TomTomStaticImage = ""
	config.Data.TomTomStaticImage.BaseURL = "https://example.com/map/1/staticimage"
	config.Data.TomTomStaticImage.TimeoutMS = 1500
	config.Data.TomTomStaticImage.RetryCount = 2
	staticPreviewSleepFn = func(time.Duration) {}

	attempts := 0
	staticPreviewHTTPClientFactory = func(timeout time.Duration) *http.Client {
		if timeout != 1500*time.Millisecond {
			t.Fatalf("expected timeout 1500ms, got %s", timeout)
		}
		return &http.Client{
			Timeout: timeout,
			Transport: staticPreviewRoundTripper(func(r *http.Request) (*http.Response, error) {
				attempts++
				if attempts < 3 {
					return nil, io.ErrUnexpectedEOF
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok")),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}

	payload, err := fetchTomTomStaticMapImage(42.980588, -81.6318773)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if string(payload) != "ok" {
		t.Fatalf("expected payload ok, got %q", string(payload))
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (1 initial + 2 retries), got %d", attempts)
	}
}

func TestFetchTomTomStaticMapImageDoesNotRetryBadRequest(t *testing.T) {
	originalFactory := staticPreviewHTTPClientFactory
	originalSleep := staticPreviewSleepFn
	originalTomTomStaticImage := config.Data.TomTomStaticImage
	originalTomTomMapKey := config.Data.APIKEY.TomTomMap
	originalTomTomStaticImageKey := config.Data.APIKEY.TomTomStaticImage
	defer func() {
		staticPreviewHTTPClientFactory = originalFactory
		staticPreviewSleepFn = originalSleep
		config.Data.TomTomStaticImage = originalTomTomStaticImage
		config.Data.APIKEY.TomTomMap = originalTomTomMapKey
		config.Data.APIKEY.TomTomStaticImage = originalTomTomStaticImageKey
	}()

	config.Data.APIKEY.TomTomMap = "map-key"
	config.Data.APIKEY.TomTomStaticImage = ""
	config.Data.TomTomStaticImage.BaseURL = "https://example.com/map/1/staticimage"
	config.Data.TomTomStaticImage.TimeoutMS = 1000
	config.Data.TomTomStaticImage.RetryCount = 3
	staticPreviewSleepFn = func(time.Duration) {}

	attempts := 0
	staticPreviewHTTPClientFactory = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: staticPreviewRoundTripper(func(r *http.Request) (*http.Response, error) {
				attempts++
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader("bad request")),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}

	_, err := fetchTomTomStaticMapImage(22.3, 114.2)
	if err == nil {
		t.Fatalf("expected error on 400 response")
	}
	if attempts != 1 {
		t.Fatalf("expected no retries for 400 response, got %d attempts", attempts)
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("expected status 400 error, got %v", err)
	}
}
