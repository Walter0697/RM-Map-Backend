package service

import (
	"mapmarker/backend/database/dbmodel"
	"testing"
	"time"
)

func TestCanTransitionCalendarSyncStatus(t *testing.T) {
	cases := []struct {
		from string
		to   string
		ok   bool
	}{
		{dbmodel.CalendarSyncStatusPending, dbmodel.CalendarSyncStatusSynced, true},
		{dbmodel.CalendarSyncStatusPending, dbmodel.CalendarSyncStatusFailed, true},
		{dbmodel.CalendarSyncStatusPending, dbmodel.CalendarSyncStatusDisconnected, true},
		{dbmodel.CalendarSyncStatusSynced, dbmodel.CalendarSyncStatusPending, true},
		{dbmodel.CalendarSyncStatusFailed, dbmodel.CalendarSyncStatusSynced, false},
		{dbmodel.CalendarSyncStatusDisconnected, dbmodel.CalendarSyncStatusSynced, false},
	}

	for _, item := range cases {
		if got := canTransitionCalendarSyncStatus(item.from, item.to); got != item.ok {
			t.Fatalf("unexpected transition result for %s -> %s: got=%v want=%v", item.from, item.to, got, item.ok)
		}
	}
}

func TestCalendarSyncRetryBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, time.Minute},
		{2, 2 * time.Minute},
		{3, 5 * time.Minute},
		{4, 15 * time.Minute},
		{9, 30 * time.Minute},
	}
	for _, item := range cases {
		if got := calendarSyncRetryBackoff(item.attempt); got != item.want {
			t.Fatalf("unexpected backoff for attempt=%d got=%s want=%s", item.attempt, got, item.want)
		}
	}
}

func TestCanTransitionCalendarConnectionStatus(t *testing.T) {
	cases := []struct {
		from string
		to   string
		ok   bool
	}{
		{dbmodel.CalendarConnectionStatusActive, dbmodel.CalendarConnectionStatusReauthorizationNeeded, true},
		{dbmodel.CalendarConnectionStatusActive, dbmodel.CalendarConnectionStatusDisconnected, true},
		{dbmodel.CalendarConnectionStatusReauthorizationNeeded, dbmodel.CalendarConnectionStatusActive, true},
		{dbmodel.CalendarConnectionStatusReauthorizationNeeded, dbmodel.CalendarConnectionStatusDisconnected, true},
		{dbmodel.CalendarConnectionStatusDisconnected, dbmodel.CalendarConnectionStatusActive, true},
		{dbmodel.CalendarConnectionStatusDisconnected, dbmodel.CalendarConnectionStatusReauthorizationNeeded, false},
	}
	for _, item := range cases {
		if got := canTransitionCalendarConnectionStatus(item.from, item.to); got != item.ok {
			t.Fatalf("unexpected connection transition for %s -> %s: got=%v want=%v", item.from, item.to, got, item.ok)
		}
	}
}

func TestNormalizeCalendarScopes(t *testing.T) {
	input := []string{"", "a", "a", "  b ", " "}
	output := normalizeCalendarScopes(input)
	if len(output) != 2 {
		t.Fatalf("expected 2 scopes, got %d (%v)", len(output), output)
	}
	if output[0] != "a" || output[1] != "b" {
		t.Fatalf("unexpected scope output %v", output)
	}
}
