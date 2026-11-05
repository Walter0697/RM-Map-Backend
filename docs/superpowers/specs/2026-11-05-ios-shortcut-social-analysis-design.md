# iOS Shortcut Social Media Analysis Design

## Goal

Allow an authenticated Roroadmap user to install an iOS Shortcut from the
Roroadmap Settings page and submit social-media URLs directly for analysis,
without relying on Telegram.

The first version is intentionally fire-and-forget. After submitting a URL,
the Shortcut displays `Feature requested — analysing.` The Shortcut does not
poll for job status or wait for marker creation.

## Scope

This change adds a native Shortcut submission path for the existing social
media analysis job pipeline. It does not change scraping, agent processing,
Google Places verification, marker creation, or Telegram behavior.

The existing Telegram integration remains available for users who need it.

## User flow

1. An authenticated user opens Roroadmap Settings.
2. The user selects **Install Roroadmap Shortcut**.
3. Roroadmap creates a short-lived, single-use setup token associated with the
   user's relation.
4. The Settings page downloads the generic Shortcut and launches its setup
   action with the setup URL.
5. The Shortcut exchanges the setup token for a dedicated API key scoped to
   social-media analysis and the submission endpoint.
6. The Shortcut stores that endpoint and key locally on the iPhone.
7. From the iOS Share Sheet, the user runs the Shortcut with a URL.
8. The Shortcut submits the URL and displays `Feature requested — analysing.`

The Shortcut does not expose the API key in the UI after setup and does not
make a follow-up request for job status.

## Architecture

### Setup token

The backend creates a random setup token when the authenticated user starts
installation. Only a hash is persisted. The token:

- expires after 10 minutes;
- can be consumed once;
- is associated with the authenticated user's relation;
- is never logged or placed in a page response other than the setup URL.

The setup exchange creates a dedicated API key with the existing API-key
model. The key receives the social-media-analysis scope and is associated with
the user's relation so the resulting marker and job remain relation-scoped.

### Backend endpoints

The existing analysis endpoint remains the submission contract:

```text
POST /integration/social-media/analyses
X-API-Key: <api-key>
Content-Type: application/json

{"url":"<shared URL>"}
```

The endpoint returns `202 Accepted` with the existing analysis job response.
The Shortcut treats any successful `2xx` response as accepted and displays the
confirmation message without parsing or showing analysis details.

The new setup exchange follows the TubeMin pattern:

```text
POST /api/shortcut/setup/<token>
```

Response:

```json
{
  "submit_url": "https://<roroadmap-host>/integration/social-media/analyses",
  "api_key": "<plaintext key>"
}
```

The setup endpoint consumes the token before returning the key. Invalid,
expired, or reused tokens return `401` and do not create an API key.

The API key is returned only to the Shortcut during setup. It is stored on the
device by the Shortcut and remains revocable through the existing API-key
management flow.

### Shortcut artifact

The Shortcut should be generic and configuration-driven, matching TubeMin's
behavior. It must not contain a permanent Roroadmap key or rely on a hardcoded
development URL.

Its setup behavior receives the one-time setup URL, calls the setup endpoint,
and stores the returned `submit_url` and `api_key` locally. Its share-sheet
behavior:

1. accepts the shared URL;
2. sends a JSON POST to the stored `submit_url`;
3. sends the stored API key using the API's expected authentication header;
4. shows `Feature requested — analysing.` on a successful `2xx` response;
5. shows a short generic error if submission fails.

The first implementation should reuse the proven TubeMin Shortcut structure
where compatible. The signed/binary artifact must not be modified by string
replacement or have a secret embedded into it. If Roroadmap needs a branded
copy, the display name can be changed in the Shortcut installation flow or a
separately signed artifact can be supplied later.

### Settings UI

Add an iOS Shortcut section to the authenticated Settings page containing:

- **Install Roroadmap Shortcut**;
- brief setup instructions;
- setup/download failure feedback;
- a note that the Shortcut has its own revocable API key.

The install flow should support iPhone Safari. The page may use a native
`shortcuts://run-shortcut` launch after downloading or sharing the artifact,
but must preserve a normal download fallback for browsers that cannot launch
the URL scheme.

The UI should not display the plaintext API key during the normal setup flow.
Existing API-key listing and revoke controls remain the source of truth for
managing the generated Shortcut key.

## Security and correctness

- Require the existing authenticated Settings session and CSRF protection to
  create setup tokens.
- Hash setup tokens at rest and delete them atomically on successful use.
- Give generated keys only the social-media-analysis scope.
- Associate generated keys with the authenticated user's relation.
- Do not put API keys in URLs, HTML, logs, analytics attributes, or Phoenix
  span attributes.
- Construct endpoint origins from trusted deployment configuration or
  sanitized forwarded headers; do not allow arbitrary user-provided origins.
- Treat duplicate submissions according to the existing analysis-job behavior;
  the Shortcut does not retry automatically in this phase.
- Keep Telegram routes and settings unchanged.

## Error behavior

The Shortcut only needs two user-visible outcomes:

- successful submission: `Feature requested — analysing.`;
- any setup or submission failure: a concise retry-oriented error.

The backend continues to return structured integration errors for diagnostics
and frontend clients. The Shortcut should not expose internal error details,
API keys, runner IDs, or trace IDs.

## Testing

Backend tests should cover:

- authenticated setup-token creation and CSRF rejection;
- token expiry and single-use consumption;
- setup exchange creates a relation-owned key with only the required scope;
- invalid setup tokens do not create keys;
- setup response returns the social-analysis endpoint and plaintext key;
- analysis submission with the generated key returns `202 Accepted`;
- generated keys can be revoked through the existing API-key flow;
- API keys and setup tokens are absent from logs and observability attributes.

Frontend tests should cover:

- the Settings install control is rendered for authenticated users;
- setup/download errors are shown without secret values;
- the success state communicates that analysis was requested rather than
  claiming that a marker already exists.

Manual acceptance should install the Shortcut on an iPhone, submit a URL from
the Share Sheet, confirm the message, and verify that a queued analysis job is
created and later produces a marker.

## Explicitly deferred

- Shortcut polling or progress updates;
- displaying the created marker inside the Shortcut;
- Telegram removal;
- Threads-specific Shortcut behavior;
- Jev integration;
- image delivery and MinIO remediation;
- a separate branded Shortcut binary if the generic artifact is sufficient.
