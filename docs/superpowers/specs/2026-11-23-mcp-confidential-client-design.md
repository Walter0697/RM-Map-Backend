# MCP confidential OAuth client design

## Goal

Allow hosted MCP clients such as Gemini, which require an OAuth client ID and
client secret, to connect to native `/mcp` without breaking the existing public
OAuth/DCR flow used by ChatGPT and other clients.

## Configuration

Add one optional confidential client configured through the backend environment
(normally injected by Infisical):

```env
MCP_OAUTH_CONFIDENTIAL_CLIENT_ID=...
MCP_OAUTH_CONFIDENTIAL_CLIENT_SECRET=...
MCP_OAUTH_CONFIDENTIAL_REDIRECT_URIS=https://client.example/callback,https://other.example/callback
```

The client ID and secret must be configured together. Redirect URIs are an exact
comma- or newline-separated allowlist; wildcards, fragments, credentials, and
non-HTTPS/non-loopback redirects remain invalid. The secret is never logged or
returned by an endpoint.

## OAuth behavior

- Discovery advertises `none` for public clients and, when configured,
  `client_secret_basic` and `client_secret_post` for the confidential client.
- The configured client is accepted by authorization only when its redirect URI
  is in the configured allowlist.
- Token exchange accepts the configured secret through HTTP Basic or the
  `client_secret` form field, using constant-time comparison.
- When the configured user, relation, and scope ceiling are present, the
  confidential client is pre-authorized: the backend validates that active
  relation and issues the code without browser login or consent.
- Dynamically registered clients remain public and continue to use PKCE without
  a secret.
- Existing authorization-code, refresh-token, consent, scope, relation, and
  resource checks remain unchanged.

## Security boundaries

The confidential client is one server-side integration credential, not a user
identity. User identity and map/relation selection still come from the existing
browser OIDC consent flow. A separate client credential should be configured for
each hosted provider when practical; multiple redirect URIs are supported for
the configured client for providers that share one credential.

## Testing

Add coverage for discovery, multiple configured redirect URIs, authorization
allow/deny behavior, Basic and form secret authentication, wrong/missing secret,
public DCR compatibility, and prevention of secret disclosure. Run the complete
backend test suite before opening the PR.
