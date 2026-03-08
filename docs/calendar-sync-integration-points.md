# Calendar Sync Integration Points

This note documents where schedule create/update/delete flows can enqueue calendar sync work without blocking existing schedule persistence.

## Existing schedule mutation paths

- GraphQL create schedule:
  - `graph/schema.resolvers.go` -> `CreateSchedule`
  - delegates to `service.CreateSchedule`
- GraphQL edit schedule:
  - `graph/schema.resolvers.go` -> `EditSchedule`
  - delegates to `service.EditSchedule`
- GraphQL remove schedule:
  - `graph/schema.resolvers.go` -> `RemoveSchedule`
  - delegates to `service.RemoveSchedule`
- Integration API create schedule:
  - `service/api_key_http.go` -> `IntegrationCreateScheduleHandler`
  - delegates to `service.CreateSchedule`

## Sync hook boundaries

- Write path remains in schedule services (`service/schedule.go`).
- Calendar sync trigger is an orchestration concern:
  - resolve provider adapter through `CalendarProviderRegistry`
  - enqueue async create/update/delete sync jobs after DB write success
- Link state persistence is stored in `schedule_calendar_sync_links`.
- Provider connection state is stored in `calendar_provider_connections`.

## Guardrails

- Never block schedule write success on provider network calls.
- Resolve and enqueue sync only after schedule transaction commits.
- Treat missing provider connection as non-fatal for schedule mutation; return explicit sync status separately.
