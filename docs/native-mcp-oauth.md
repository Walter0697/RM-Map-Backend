# Native MCP with OAuth

The backend can host the existing 22 Marker Map tools directly at `/mcp` using
stateless Streamable HTTP. No Node sidecar, backend URL header, shared administrator
key, or loopback HTTP call is involved. Existing tool names and arguments remain compatible with RM-Map-MCP. The native
nearby tools also expose the new arguments documented below. Existing `/integration` API keys, scripts and the
separate MCP service remain supported during migration.

## Deployment prerequisite: verified identities

**Do not deploy without reading [OIDC identity migration](oidc-identity-migration.md).**
This release hardens browser OIDC too. Previously username-based accounts must be
explicitly bound to a verified provider issuer and immutable subject before their
next login. No email/username auto-linking is performed. Existing IDs, roles,
relations, API keys and browser-session records are retained. Back up the DB,
stage mappings and test a normal login before switching traffic.

The new binary also provides an operator-only enrollment command:

```sh
./backend bind-oidc EXISTING_USER_ID EXACT_ISSUER VERIFIED_SUBJECT
```

Run with the normal config/database connectivity. Independently verify the subject
in your provider's administrator console. This command refuses disabled users or
reassignment of an identity already owned by another user. It is not an HTTP API.

## Enable

```env
MCP_ENABLED=true
MCP_PUBLIC_ORIGIN=https://your-marker-backend.example
```

The public origin must be HTTPS, with no path, credentials, query or fragment.
Configure the existing OIDC provider client and backend callback normally. Keep
the same backend hostname throughout authorization and callback. Configure sticky
routing if multiple replicas handle OIDC's short-lived in-memory login state.
MCP token/grant state is in PostgreSQL, not process memory.

Add `https://your-marker-backend.example/mcp` to your MCP client. Discovery:

- `/.well-known/oauth-protected-resource/mcp`
- `/.well-known/oauth-authorization-server`

The client registers exact redirect URIs, starts S256 authorization with
`resource=https://your-marker-backend.example/mcp`, and sends that same resource
on code exchange and refresh. HTTPS callbacks and literal loopback HTTP callbacks
are supported. No wildcard redirects, implicit flow, passwords or client secrets.

Every connection requires fresh verified provider login and explicit consent.
The person chooses an existing shared map/relation. Default scopes are read-only;
schedule/travel-plan writes and geocoding require the requested additional scope.
Existing roles and testing-data visibility still constrain operations. Consent
does not upgrade a role. Active user status and relation membership are rechecked
for every request and refresh, so removing membership cuts off access immediately.

## Tokens and disconnect

- Access tokens are opaque, last 10 minutes and are audience-bound to `/mcp`.
- Refresh tokens rotate; reusing one revokes its entire connection family.
- Connections expire after 30 days and require a new login/consent.
- Only token hashes are stored. Provider/browser tokens are not MCP credentials.
- `POST /mcp/oauth/revoke` accepts `token` and its `client_id`.
- `GET /mcp/connections` with the connection's bearer token shows that connection.
- `DELETE /mcp/connections` disconnects that same connection.

Deploy behind normal request/body/rate limits, especially dynamic registration and
authorization. Registration is capped at 1,000 stored clients; administrators
should review/remove abandoned registrations through their database administration
process. Expired authorization/token rows are retained for audit/replay detection;
plan routine retention maintenance, never purge active token-family history.

## Compatibility and rollout

Keep the old MCP service until a real client has completed discovery, consent,
read operations, an explicitly approved write, refresh and disconnect. Clients
must migrate explicitly to the native URL; existing API keys are not converted to
OAuth grants. There are no runner changes. Returning to the old MCP service does
not require removing new database tables. See the identity guide for image
rollback security consequences.

The native tools fix the old MCP's missing marker/schedule detail endpoints by
performing relation-scoped reads in-process. Travel-plan item tools preserve the
old whole-plan read/replace behavior; avoid concurrent writers to the same plan
until optimistic concurrency is introduced separately.

## Find saved pins near a place

For coordinates, call `list_markers_nearby` with
`{"latitude":43.775,"longitude":-79.347,"radius_meters":3000,"exclude_type":"food","limit":10}`.
The distance is straight-line, not walking distance. Use `radius_meters: 3000`
for 3 km. Legacy `radius` and `area` still mean meters; when several are supplied,
the first in `radius_meters`, `radius`, `area` order takes precedence.
Both nearby tools support optional exact `type` and `exclude_type`, applied
before the result limit. Use `list_marker_types` for values.

To search by name in one call, use
`{"place":"Don Mills Station, Toronto","place_country_code":"CA","radius_meters":3000}`.
Named lookup sends the supplied place text and optional country code to the existing
TomTom provider and requires the existing static preview scope and a configured
TomTom key in addition to marker-read permission. Coordinates do not require
geocoding permission. Do not provide a name and coordinates together.

The response preserves the existing items and pagination fields and adds
`origin`, `radius_meters`, and `filters` (even with no matching pins). The
`status` is `ok` for a completed pin search. If the provider returns multiple
candidates, the status is `place_ambiguous` and the response includes up to five
candidates. Ask for the intended place or retry with known coordinates; no pins
are queried until there is a single origin. `place_not_found` means the origin
was not resolved, not that there are no saved pins nearby. If geocoding is
unavailable, use coordinates. `search_nearby_markers`, the matching/deduplication
tool, remains coordinate-based.

## Link back to saved markers

Use each result's `roroadmap_url` as the primary link in marker listings.
It opens the saved marker in RoroadMap and does not grant access by itself.
If missing, show the marker name as text. The optional `source_url` is an
external source (for example Instagram); show it separately only on request.
The existing `link` and `social_media_link` fields remain unchanged.

The backend uses the frontend origin from `oidc.frontendredirecturl`, then
`telegramnotification.frontendurl` (or `TELEGRAM_FRONTEND_URL`), then a single
literal `app.allowedorigin`. If none is usable, the map link is omitted.

## Tests

```sh
go test -race ./...
MCP_TEST_DATABASE_URL='postgres://postgres:test-only@localhost:5432/marker_mcp_test?sslmode=disable' go test -race ./service -run TestMCPOAuthDatabaseSecurity
```

Use a disposable database. The lifecycle test creates and drops its own schema;
the PostgreSQL account must have schema-creation permission. CI supplies a fresh
PostgreSQL service and runs this test, rather than silently skipping it.

Test invalid signature/issuer/audience/expiry/nonce, browser state replay, PKCE,
wrong resource/redirect, code replay, refresh replay, disconnect, disabled users,
cross-relation requests, insufficient scopes, and preserved legacy API-key roles.
