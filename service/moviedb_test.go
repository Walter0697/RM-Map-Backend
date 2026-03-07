package service

import (
	"net/url"
	"strings"
	"testing"
)

func TestSearchByNameEncodesQueryAndUsesAuditWrapper(t *testing.T) {
	original := movieDBGetRequestWithAuditFn
	defer func() {
		movieDBGetRequestWithAuditFn = original
	}()

	var capturedProvider string
	var capturedOperation string
	var capturedURL string
	movieDBGetRequestWithAuditFn = func(provider string, operation string, rawURL string, validateBody func([]byte) error) ([]byte, error) {
		capturedProvider = provider
		capturedOperation = operation
		capturedURL = rawURL
		return []byte(`{"page":1,"results":[]}`), nil
	}

	query := "Inside Out 2 & test/space"
	_, err := SearchByName(query)
	if err != nil {
		t.Fatalf("SearchByName returned error: %v", err)
	}

	if capturedProvider != ExternalAPIProviderMovieDB {
		t.Fatalf("expected provider %s, got %s", ExternalAPIProviderMovieDB, capturedProvider)
	}
	if capturedOperation != "search_movie" {
		t.Fatalf("expected operation search_movie, got %s", capturedOperation)
	}
	if strings.Contains(capturedURL, " ") {
		t.Fatalf("expected encoded URL without spaces, got %s", capturedURL)
	}

	encoded := url.QueryEscape(query)
	if !strings.Contains(capturedURL, "query="+encoded) {
		t.Fatalf("expected encoded query in URL, got %s", capturedURL)
	}
}

