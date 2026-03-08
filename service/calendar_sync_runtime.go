package service

import (
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"sync"
)

var (
	calendarSyncRuntimeOnce sync.Once
	calendarRuntimeState    *calendarSyncRuntimeState
)

type calendarSyncRuntimeState struct {
	registry      *CalendarProviderRegistry
	orchestrator  *CalendarSyncOrchestrator
	connectionSvc *CalendarConnectionService
	linkRepo      *CalendarSyncLinkRepository
	jobQueue      *CalendarSyncJobQueue
}

func getCalendarSyncRuntime() *calendarSyncRuntimeState {
	calendarSyncRuntimeOnce.Do(func() {
		registry, _ := NewCalendarProviderRegistry()
		if config.Data.CalendarGoogle.Enable {
			_ = registry.Register(NewGoogleCalendarAdapter())
		}
		calendarRuntimeState = &calendarSyncRuntimeState{
			registry:      registry,
			orchestrator:  NewCalendarSyncOrchestrator(registry),
			connectionSvc: NewCalendarConnectionService(database.Connection),
			linkRepo:      NewCalendarSyncLinkRepository(database.Connection),
			jobQueue:      NewCalendarSyncJobQueue(database.Connection),
		}
	})
	return calendarRuntimeState
}
