package service

import (
	"context"
	"errors"
	"io"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGoogleCalendarAdapterCreateUpdateDelete(t *testing.T) {
	received := make([]string, 0)
	transport := googleRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		received = append(received, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/calendars/primary/events":
			return jsonResponse(http.StatusOK, `{"id":"evt-1"}`), nil
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/calendars/primary/events/"):
			return jsonResponse(http.StatusOK, `{"id":"evt-1"}`), nil
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/calendars/primary/events/"):
			return jsonResponse(http.StatusNoContent, ``), nil
		default:
			return jsonResponse(http.StatusNotFound, `{}`), nil
		}
	})

	originalBase := config.Data.CalendarGoogle.APIBaseURL
	config.Data.CalendarGoogle.APIBaseURL = "https://mock.google.test"
	defer func() { config.Data.CalendarGoogle.APIBaseURL = originalBase }()

	encryptedAccess, err := encryptCalendarSecret("token-1")
	if err != nil {
		t.Fatalf("failed to encrypt access token: %v", err)
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	connection := dbmodel.CalendarProviderConnection{
		AccessTokenEncrypted: encryptedAccess,
		TokenExpiresAt:       &expires,
	}

	adapter := NewGoogleCalendarAdapter()
	adapter.httpClient = &http.Client{Transport: transport}
	result, err := adapter.CreateEvent(context.Background(), connection, CalendarEventUpsertRequest{
		Title:       "A",
		Description: "B",
		StartAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}
	if strings.TrimSpace(result.ExternalEventID) != "evt-1" {
		t.Fatalf("unexpected external event id: %s", result.ExternalEventID)
	}
	if err := adapter.UpdateEvent(context.Background(), connection, "evt-1", CalendarEventUpsertRequest{
		Title:   "A2",
		StartAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("update event failed: %v", err)
	}
	if err := adapter.DeleteEvent(context.Background(), connection, "evt-1"); err != nil {
		t.Fatalf("delete event failed: %v", err)
	}
	if len(received) != 3 {
		t.Fatalf("expected 3 api calls, got %d", len(received))
	}
}

func TestGoogleCalendarAdapterRevokedTokenRequiresReauth(t *testing.T) {
	transport := googleRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnauthorized, `{"error":"invalid_grant"}`), nil
	})

	originalBase := config.Data.CalendarGoogle.APIBaseURL
	config.Data.CalendarGoogle.APIBaseURL = "https://mock.google.test"
	defer func() { config.Data.CalendarGoogle.APIBaseURL = originalBase }()

	encryptedAccess, err := encryptCalendarSecret("token-1")
	if err != nil {
		t.Fatalf("failed to encrypt access token: %v", err)
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	connection := dbmodel.CalendarProviderConnection{
		AccessTokenEncrypted: encryptedAccess,
		TokenExpiresAt:       &expires,
	}

	adapter := NewGoogleCalendarAdapter()
	adapter.httpClient = &http.Client{Transport: transport}
	err = adapter.UpdateEvent(context.Background(), connection, "evt-1", CalendarEventUpsertRequest{
		Title:   "A2",
		StartAt: time.Now().UTC(),
	})
	var providerErr *CalendarProviderOperationError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected provider operation error, got %v", err)
	}
	if !providerErr.ReauthRequired {
		t.Fatalf("expected reauthorization-required flag")
	}
}

type googleRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn googleRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
