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

The worker accepts public HTTP(S) URLs, synchronously acquires and normalizes
the source, then submits the normalized content to wcodex-runner. Instagram
and Threads use the configured Apify actor; other URLs use the bounded generic
HTML fetcher. Both paths expose the same title, description, hashtags, image
references, video references, and metadata to the agent.

When a source exposes a direct canonical image (for example through
`og:image`), the structured analysis may return `image_url`. Backend-created
markers pass that value to the existing image downloader. The production and
non-production n8n workflows use the same optional value as `image_link`, with
the normalized media preview URL as a fallback. Missing or unusable image URLs
do not change the marker's other metadata.

For marker creation, the agent also returns optional `tags`, `price`,
`need_booking`, `estimate_time`, `from_time`, and `to_time` values. Tags are
normalized into `#tag` tokens in the description, which is the existing source
used by the hashtag listing endpoint.

## Runtime configuration

Non-secret settings belong in `[socialmediaanalysis]` or environment overrides:

- `WCODEX_RUNNER_URL`
- `SOCIAL_MEDIA_ANALYSIS_ENABLE`
- `SOCIAL_MEDIA_ANALYSIS_RUNNER_TIMEOUT_SECONDS`
- `SOCIAL_MEDIA_ANALYSIS_SOURCE_MAX_BYTES`

The `[infisical]` section supports Infisical Universal Auth and local TOML
fallbacks. Infisical values take precedence, followed by explicitly named
environment overrides, followed by TOML values. Configure these Infisical
secrets for source-specific Apify scraping:

- `APIFY_TOKEN`
- `APIFY_INSTAGRAM_ACTOR_ID` (`apify~instagram-scraper`)
- `APIFY_THREADS_ACTOR_ID` (`n7ppgtM2MF7vWvKad`)
- `GOOGLE_MAPS_API_KEY` when Google Places verification is enabled

`APIFY_ACTOR_ID` remains supported as a legacy fallback. `apifybaseurl` can be
used to point at a compatible Apify API endpoint. Gemini credentials can be
configured now for a later media-provider phase; Gemini is not invoked by the
scraping phase.

Google Maps verification is optional. Enable it in `config.toml` with
`[googlemaps] enable=true`, then configure `GOOGLE_MAPS_API_KEY` in Infisical
project `roroadmap`, environment `dev`, secret path `/roroadmap`. The key may
also be supplied through the configured environment override or
`[googlemaps] apikey` for local-only fallback testing. The worker first checks
the agent's proposed location against Google Places results; an unambiguous
match uses the verified result, while an ambiguous or missing match is sent to
a second wcodex comparison job. If Google is disabled, unavailable, or not
configured, the original agent-only marker flow is preserved.

## Observability and artifacts

Backend provider traces are exported to the Phoenix project
`roroadmap-social-analysis`. The wcodex-runner agent trace remains in its
existing `wcodex` Phoenix project. Use the analysis job ID and runner job ID to
correlate the two projects. Backend spans contain provider names, operations,
durations, statuses, bounded error summaries, Apify run IDs, Google candidate
counts, and MinIO object references. They do not contain provider keys, raw
authorization headers, full videos, or unrestricted provider responses.

Enable the backend exporter with:

```toml
[phoenix]
enable=true
endpoint="http://phoenix.home"
project="roroadmap-social-analysis"
apikeyenv="PHOENIX_API_KEY"

[minio]
enable=true
endpoint="http://minio.home:9000"
region="us-east-1"
bucket="roroadmap-social-analysis"
accesskeyenv="MINIO_ACCESS_KEY"
secretkeyenv="MINIO_SECRET_KEY"
```

Store these secrets in Infisical under the matching project/environment/path
(for example, project `roroadmap`, environment `dev`, path `/roroadmap`):

- `PHOENIX_API_KEY`
- `MINIO_ACCESS_KEY`
- `MINIO_SECRET_KEY`

Use separate values, Phoenix projects, and MinIO buckets for `dev` and `prod`.
MinIO objects are private and job-scoped. Source snapshots and bounded first
image/video artifacts are stored under
`social-analysis/{analysis_job_id}/`; the database stores their checksums and
metadata. Phoenix stores references and checksums only. Phoenix or MinIO being
unavailable does not fail ordinary analysis when artifact capture is optional.

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
