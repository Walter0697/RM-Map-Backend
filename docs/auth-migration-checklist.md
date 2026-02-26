# LDAP to OIDC Migration Checklist

1. Set `[app].authmode="oidc"` in `config.toml`.
2. Set `[oidc].enable=true` and populate:
- `issuer`
- `clientid`
- `clientsecret`
- `redirecturl`
- `frontendredirecturl`
3. Keep `[ldap]` values only during transition. They are now ignored when auth mode is `oidc`.
4. Verify backend auth health:
- `GET /auth/health` should return `status: ok` and `mode: oidc`.
5. Verify login redirect flow:
- Open frontend `/login`.
- Confirm redirect to IdP occurs.
- Confirm callback returns to `/login?token=...&username=...` and user session is established.
6. Confirm local fallback behavior:
- In local/development only, set `[app].authmode="local-password"` to test password login.
- In non-local environments, `local-password` mode is rejected at startup.
7. Remove LDAP-specific operational runbooks and update on-call docs to use OIDC troubleshooting.
