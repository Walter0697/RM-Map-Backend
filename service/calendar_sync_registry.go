package service

import (
	"context"
	"errors"
	"fmt"
	"mapmarker/backend/database/dbmodel"
	"strings"
	"time"
)

var (
	ErrCalendarProviderNotFound = errors.New("calendar provider adapter not found")
	ErrCalendarProviderInvalid  = errors.New("invalid calendar provider adapter")
)

type CalendarEventUpsertRequest struct {
	Title       string
	Description string
	StartAt     time.Time
	EndAt       *time.Time
	Timezone    string
	Location    string
}

type CalendarCreateEventResult struct {
	ExternalEventID    string
	ExternalCalendarID string
}

type CalendarProviderAdapter interface {
	ProviderKey() string
	CreateEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, request CalendarEventUpsertRequest) (CalendarCreateEventResult, error)
	UpdateEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, externalEventID string, request CalendarEventUpsertRequest) error
	DeleteEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, externalEventID string) error
}

type CalendarProviderRegistry struct {
	providers map[string]CalendarProviderAdapter
}

func NewCalendarProviderRegistry(adapters ...CalendarProviderAdapter) (*CalendarProviderRegistry, error) {
	registry := &CalendarProviderRegistry{
		providers: make(map[string]CalendarProviderAdapter, len(adapters)),
	}
	for _, adapter := range adapters {
		if err := registry.Register(adapter); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (registry *CalendarProviderRegistry) Register(adapter CalendarProviderAdapter) error {
	if adapter == nil {
		return ErrCalendarProviderInvalid
	}
	key := normalizeCalendarProviderKey(adapter.ProviderKey())
	if key == "" {
		return fmt.Errorf("%w: provider key is required", ErrCalendarProviderInvalid)
	}
	if registry.providers == nil {
		registry.providers = make(map[string]CalendarProviderAdapter)
	}
	registry.providers[key] = adapter
	return nil
}

func (registry *CalendarProviderRegistry) Resolve(providerKey string) (CalendarProviderAdapter, error) {
	key := normalizeCalendarProviderKey(providerKey)
	adapter, ok := registry.providers[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrCalendarProviderNotFound, key)
	}
	return adapter, nil
}

func normalizeCalendarProviderKey(providerKey string) string {
	return strings.ToLower(strings.TrimSpace(providerKey))
}

type CalendarSyncOrchestrator struct {
	registry *CalendarProviderRegistry
}

func NewCalendarSyncOrchestrator(registry *CalendarProviderRegistry) *CalendarSyncOrchestrator {
	return &CalendarSyncOrchestrator{registry: registry}
}

func (orchestrator *CalendarSyncOrchestrator) ResolveAdapter(providerKey string) (CalendarProviderAdapter, error) {
	if orchestrator == nil || orchestrator.registry == nil {
		return nil, ErrCalendarProviderNotFound
	}
	return orchestrator.registry.Resolve(providerKey)
}
