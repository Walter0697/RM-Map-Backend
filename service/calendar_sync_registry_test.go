package service

import (
	"context"
	"errors"
	"mapmarker/backend/database/dbmodel"
	"testing"
)

type calendarAdapterStub struct {
	key string
}

func (stub calendarAdapterStub) ProviderKey() string {
	return stub.key
}

func (stub calendarAdapterStub) CreateEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, request CalendarEventUpsertRequest) (CalendarCreateEventResult, error) {
	return CalendarCreateEventResult{}, nil
}

func (stub calendarAdapterStub) UpdateEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, externalEventID string, request CalendarEventUpsertRequest) error {
	return nil
}

func (stub calendarAdapterStub) DeleteEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, externalEventID string) error {
	return nil
}

func TestCalendarProviderRegistryResolvesByKey(t *testing.T) {
	registry, err := NewCalendarProviderRegistry(calendarAdapterStub{key: "google"})
	if err != nil {
		t.Fatalf("expected registry construction to succeed, got error: %v", err)
	}

	adapter, err := registry.Resolve(" GOOGLE ")
	if err != nil {
		t.Fatalf("expected provider to resolve, got error: %v", err)
	}

	if adapter.ProviderKey() != "google" {
		t.Fatalf("expected google adapter, got %s", adapter.ProviderKey())
	}
}

func TestCalendarProviderRegistryReturnsNotFound(t *testing.T) {
	registry, err := NewCalendarProviderRegistry(calendarAdapterStub{key: "google"})
	if err != nil {
		t.Fatalf("expected registry construction to succeed, got error: %v", err)
	}

	_, err = registry.Resolve("outlook")
	if !errors.Is(err, ErrCalendarProviderNotFound) {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestCalendarProviderRegistryRejectsInvalidAdapter(t *testing.T) {
	if _, err := NewCalendarProviderRegistry(calendarAdapterStub{key: "  "}); !errors.Is(err, ErrCalendarProviderInvalid) {
		t.Fatalf("expected invalid adapter error for blank provider key, got: %v", err)
	}
}
