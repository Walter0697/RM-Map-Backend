# Social Media Analysis

The first client-facing entry point is the authenticated integration API:

```http
POST /integration/social-media/analyses
X-API-Key: <integration key with social-media:analysis scope>
Content-Type: application/json

{"url":"https://example.com/public-post"}
```

The response is `202 Accepted` with a job ID. Poll the job with:

```http
GET /integration/social-media/analyses/{job_id}
```

The worker accepts public HTTP(S) URLs, uses the source adapter boundary, and
submits analysis work to wcodex-runner. Instagram and Threads currently use the
bounded generic fetch fallback; dedicated adapters can replace that behavior
without changing the API contract.

## Runtime configuration

Non-secret settings belong in `[socialmediaanalysis]` or environment overrides:

- `WCODEX_RUNNER_URL`
- `SOCIAL_MEDIA_ANALYSIS_ENABLE`
- `SOCIAL_MEDIA_ANALYSIS_RUNNER_TIMEOUT_SECONDS`
- `SOCIAL_MEDIA_ANALYSIS_SOURCE_MAX_BYTES`

## Secret policy

Scraper and provider API keys must be supplied through the deployment's
Infisical-backed secret injection. They must not be committed to
`config.toml`, stored in n8n workflow definitions, included in wcodex prompts,
or written to job metadata/logs.

The migration inventory should cover existing n8n scraper, OpenAI, map, and
other provider credentials. Backend code should receive only the provider
secrets required by its configured adapters. A runner job should receive a
secret capability only when the agent itself must use that secret; backend-owned
provider tools should keep credentials in the backend boundary.

## Current safety boundary

Only HTTP(S) URLs are accepted. Localhost, private-network, link-local,
metadata-service, embedded-credential, and unsupported-scheme URLs are
rejected. Fetch responses are bounded by size and must be treated as untrusted
input.
