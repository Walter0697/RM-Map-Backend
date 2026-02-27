# API Key Rollout Checklist

1. Run backend tests:
- `GOCACHE=/tmp/go-build go test ./...`
2. Verify auth mode health:
- `GET /auth/health`
3. Verify API key management endpoints with admin JWT:
- create/list/revoke/rotate
4. Verify integration endpoints with scoped API key:
- markers list/create/update
- schedules list/create
5. Verify scope denial behavior and audit log entries.
6. Verify audit retention cleanup and revoked-key cleanup behavior using configured limits.
7. Confirm OIDC/local-password user login flows are unaffected.
8. Communicate JWT automation deprecation and migration to API key auth.
