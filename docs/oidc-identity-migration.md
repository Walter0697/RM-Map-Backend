# Verified OIDC identity migration

This release stops treating a username/email or an unverified ID-token payload as
proof of identity. Login now verifies the provider signature, issuer, audience,
expiry and nonce, and binds the authorization response to the initiating browser
and a one-use, random state with S256 PKCE. Production issuers require HTTPS.

## Before rollout

Back up the database. For **each existing person**, an administrator must obtain
the exact issuer and immutable subject from the trusted identity provider's admin
console, independently verify that subject belongs to that person, and map it to
the correct existing `users.id`. Do not infer ownership from a matching username,
email, or the person supplying an arbitrary subject. Existing account IDs, roles,
relation memberships and data must not be changed.

**This is not a seamless upgrade.** Stage the binding table and verified mappings
before deploying the new image. Otherwise every unbound person will be unable to
start a new login. Existing browser sessions remain on the existing auth-state
validation path, but are not a substitute for enrolling identities.

The release's automatic migration creates this same table. On the old running
version, an administrator can first create it through a controlled migration:

```sql
CREATE TABLE IF NOT EXISTS oidc_identities (
  id bigserial PRIMARY KEY,
  issuer text NOT NULL,
  subject text NOT NULL,
  user_id bigint NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS oidc_issuer_subject
  ON oidc_identities (issuer, subject);
CREATE INDEX IF NOT EXISTS idx_oidc_identities_user_id ON oidc_identities (user_id);
```

Populate it with parameterized statements through your trusted administration
process, after independently verifying provider identities:

```sql
BEGIN;
-- Verify this is the intended, active, non-deleted local user.
SELECT id, username, is_activated FROM users
WHERE id = :verified_user_id AND is_activated = true AND deleted_at IS NULL
FOR UPDATE;
-- STOP and ROLLBACK unless exactly one intended active user was returned.
-- Insert only after independently confirming the provider's issuer and subject.
INSERT INTO oidc_identities (issuer, subject, user_id)
VALUES (:exact_provider_issuer, :verified_provider_subject, :verified_user_id);
COMMIT;
```

Use the issuer exactly as advertised in discovery, including any trailing slash.
The `(issuer, subject)` unique index prevents duplicate mappings. A collision is
an error to investigate, not a reason to overwrite another user's binding. New
accounts also need explicit provisioning and identity binding in this release.
Unbound identities fail closed with an administrator-contact message; there is
no automatic username/email fallback. Disabled/deleted local accounts cannot log
in. A provider username change does not change the bound local account.

## Compatibility and deployment

Local-development password mode, existing browser JWTs, GraphQL and integration
API keys are unchanged. Existing sessions are not silently invalidated, but MCP
OAuth consent requires fresh verified OIDC login, not a legacy browser session.
OIDC starts use a short-lived HttpOnly SameSite=Lax cookie. All login steps must
use the same backend hostname; deployments with multiple replicas need sticky
routing for this in-memory, five-minute login state. Restarting a replica cancels
in-flight login attempts; restarting login is safe. Do not route callback to a
different host. Allow the backend to fetch provider discovery and JWKS over HTTPS.
No secrets/tokens should be printed in logs or copied into migration documents.

Validate a normal login and rejected wrong-audience, wrong-issuer, wrong-nonce,
expired-token, invalid-signature, missing-cookie and replayed-state attempts.

## Bootstrap helper and rollback

The operator-only `service.BindOIDCIdentity(userID, issuer, subject)` helper
performs transactional enrollment: checks the active, non-deleted local user,
requires the exact configured issuer, and refuses to overwrite an existing
identity mapping. Calling it again for the same identity/user is idempotent.
The backend exposes this through an operator CLI, not an HTTP route:

```sh
backend bind-oidc <existing-user-id> '<exact-configured-issuer>' '<verified-subject>'
```

Use the actual backend executable path for your installation and its normal
configuration/database settings. The command runs database migrations, enrolls
the mapping, then exits without starting HTTP or background workers. It is
idempotent for the same mapping. Run it from the new image as a one-off bootstrap
before replacing the serving image, or use the pre-staging SQL above. Do not run
it against an unintended database. It does not discover or guess the subject.

Automated coverage: `TestOIDCPreStagedPostgresMigration` executes the documented
pre-staging DDL verbatim in a disposable PostgreSQL schema, runs the GORM migration
twice, verifies preserved account identity/roles, refused reassignment and
username auto-linking, disabled/deleted rejection, and both existing and newly
issued browser session validation. It runs when `MCP_TEST_DATABASE_URL` is set;
without that variable it explicitly skips, rather than claiming DB coverage.

Before deploying, compare the staged mapping list with your provider admin list
and verify every person expected to log in is enrolled. Keep database backups.
If a subject was typed incorrectly, stop rollout and correct it through an
audited administrator operation after re-verification, never match on email.
If deployment must roll back, reverting the application image is non-destructive:
the additive identity table can remain, and user IDs, roles, API keys, relations
and browser session records have not been rewritten. The old image restores its
old weaker OIDC verification, so rollback is temporary and should be treated as a
security regression rather than a permanent compatibility mode. Do not delete
the identity mappings or revert users' data as part of rollback.
