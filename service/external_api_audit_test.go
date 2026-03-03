package service

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"mapmarker/backend/database/dbmodel"
)

type externalAPIRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn externalAPIRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestGetRequestWithExternalAPIAuditSuccess(t *testing.T) {
	captured := make([]ExternalAPIAuditWriteInput, 0, 1)
	createExternalAPIAuditEventFn = func(input ExternalAPIAuditWriteInput) error {
		captured = append(captured, input)
		return nil
	}
	externalAPIHTTPClientFactory = func() *http.Client {
		return &http.Client{
			Transport: externalAPIRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}
	t.Cleanup(func() {
		createExternalAPIAuditEventFn = CreateExternalAPIAuditEvent
		externalAPIHTTPClientFactory = func() *http.Client {
			return &http.Client{Timeout: 10 * time.Second}
		}
	})

	body, err := GetRequestWithExternalAPIAudit(ExternalAPIProviderTomTomMap, "reverse_geocode", "https://example.local/test", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected body %q", string(body))
	}
	if len(captured) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(captured))
	}
	if captured[0].StatusClass != dbmodel.ExternalAPIAuditStatusSuccess {
		t.Fatalf("expected success status, got %s", captured[0].StatusClass)
	}
}

func TestGetRequestWithExternalAPIAuditDecodeError(t *testing.T) {
	captured := make([]ExternalAPIAuditWriteInput, 0, 1)
	createExternalAPIAuditEventFn = func(input ExternalAPIAuditWriteInput) error {
		captured = append(captured, input)
		return nil
	}
	externalAPIHTTPClientFactory = func() *http.Client {
		return &http.Client{
			Transport: externalAPIRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"items":[`)),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}
	t.Cleanup(func() {
		createExternalAPIAuditEventFn = CreateExternalAPIAuditEvent
		externalAPIHTTPClientFactory = func() *http.Client {
			return &http.Client{Timeout: 10 * time.Second}
		}
	})

	_, err := GetRequestWithExternalAPIAudit(ExternalAPIProviderMovieDB, "search_movie", "https://example.local/test", func(body []byte) error {
		return fmt.Errorf("decode response: %s", string(body))
	})
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if len(captured) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(captured))
	}
	event := captured[0]
	if event.StatusClass != dbmodel.ExternalAPIAuditStatusError {
		t.Fatalf("expected error status, got %s", event.StatusClass)
	}
	if event.ErrorClass != externalAPIErrorClassDecode {
		t.Fatalf("expected decode error class, got %s", event.ErrorClass)
	}
	// Guardrail: payloads are never persisted as dedicated fields in the audit model.
	if strings.Contains(strings.ToLower(event.ErrorDetail), "apikey") || strings.Contains(strings.ToLower(event.ErrorDetail), "token") {
		t.Fatalf("unexpected credential-like content in error detail: %s", event.ErrorDetail)
	}
}

func TestParseExternalAPIUsageFilterValidation(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	_, err := parseExternalAPIUsageFilter(map[string]string{
		"from":     "bad-time",
		"to":       now.Format(time.RFC3339),
		"provider": "",
	}, now)
	if err == nil {
		t.Fatalf("expected invalid from error")
	}

	_, err = parseExternalAPIUsageFilter(map[string]string{
		"from":     now.Add(1 * time.Hour).Format(time.RFC3339),
		"to":       now.Format(time.RFC3339),
		"provider": "",
	}, now)
	if err == nil {
		t.Fatalf("expected from before to error")
	}

	filter, err := parseExternalAPIUsageFilter(map[string]string{
		"from":     now.Add(-1 * time.Hour).Format(time.RFC3339),
		"to":       now.Format(time.RFC3339),
		"provider": ExternalAPIProviderTomTomMap,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.Provider != ExternalAPIProviderTomTomMap {
		t.Fatalf("unexpected provider %s", filter.Provider)
	}
}
