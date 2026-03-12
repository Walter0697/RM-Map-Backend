# Local Authentik OIDC Testing

## Example Backend Config (`config.toml`)

```toml
[app]
environment="development"
authmode="oidc"
authsessionttlseconds=31536000

[oidc]
enable=true
issuer="http://localhost:9000/application/o/rmmap/"
clientid="your-client-id"
clientsecret="your-client-secret"
redirecturl="http://localhost:1998/auth/oidc/callback"
frontendredirecturl="http://localhost:3000/login"
scopes=["openid","profile","email"]
usernameclaim="preferred_username"
defaultrole="user"
sessionttlseconds=31536000
accesstokenttlseconds=31536000
refreshtokenttlseconds=31536000
```

## Repeatable Local Workflow

1. Run backend and frontend locally.
2. Verify mode endpoint:
- `curl http://localhost:1998/auth/mode`
3. Open frontend login page:
- `http://localhost:3000/login`
4. Complete Authentik login.
5. Confirm callback and JWT session are created.
6. Switch to local password flow for comparison by setting:
- `[app].authmode="local-password"`
7. Restart backend and verify password login works in development.
