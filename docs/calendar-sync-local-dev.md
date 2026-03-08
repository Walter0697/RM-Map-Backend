# Calendar Sync Local/Dev Setup

## 1) Google OAuth app setup

1. Create an OAuth client in Google Cloud Console (Web application).
2. Add redirect URI:
   - `http://localhost:1998/calendar/google/callback`
3. Grant calendar scope:
   - `https://www.googleapis.com/auth/calendar.events`

## 2) Backend config

In `config.toml`:

```toml
[calendargoogle]
enable=true
clientid="<google-client-id>"
clientsecret="<google-client-secret>"
redirecturl="http://localhost:1998/calendar/google/callback"
frontendredirecturl="http://localhost:3000/schedules"
authendpoint="https://accounts.google.com/o/oauth2/v2/auth"
tokenendpoint="https://oauth2.googleapis.com/token"
apibaseurl="https://www.googleapis.com/calendar/v3"
scopes=["https://www.googleapis.com/auth/calendar.events"]
```

`[app].jwtkey` must be set because calendar tokens are encrypted with a key derived from it.

## 3) Manual verification flow

1. Login and get a valid JWT.
2. Open:
   - `GET /calendar/google/connect` with `Authorization` header.
3. Complete consent in Google.
4. Verify:
   - `GET /calendar/providers/status`
5. Trigger sync:
   - `POST /calendar/schedules/{id}/sync-now`
6. Retry failed:
   - `POST /calendar/schedules/{id}/retry-sync`
7. Disconnect:
   - `POST /calendar/schedules/{id}/disconnect-sync`

## 4) Mock provider stubs for future providers

Use `CalendarProviderAdapter` with an in-memory stub in tests:

- Implement `ProviderKey`, `CreateEvent`, `UpdateEvent`, and `DeleteEvent`.
- Register stub in `NewCalendarProviderRegistry(...)`.
- Simulate retryable, reauthorization-required, and not-found cases via `CalendarProviderOperationError`.

Existing examples:
- `service/calendar_sync_registry_test.go`
- `service/calendar_sync_state_test.go`
