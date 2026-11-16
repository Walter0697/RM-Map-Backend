# Social Analysis Observability and Asset Storage Design

## Goal

Make social-media analysis observable across backend providers without mixing
provider work into the wcodex agent chain, while retaining useful source
artifacts in the home-server MinIO instance.

## Context

The current pipeline stores normalized source data in the analysis job, sends
Agent 1 and optional Agent 2 work to wcodex-runner, and relies on Phoenix for
the runner's agent traces. Apify and Google Places are called by the backend.
Jev is planned but is not integrated yet. Binary assets and large scraper
payloads should not be placed in Phoenix spans or prompts.

## Architecture

Each analysis receives a backend-owned correlation ID. The backend emits a
root OpenTelemetry trace to a dedicated Phoenix project,
`roroadmap-social-analysis`, with child spans for:

- source scraping (`apify` or `generic`)
- media download and preparation
- Google Places lookup
- Jev decisions when that provider is added
- submission of Agent 1 and Agent 2 to wcodex-runner

wcodex-runner continues writing its own traces to the existing `wcodex`
Phoenix project. Backend spans record the analysis job ID and runner job ID so
the two projects can be correlated without pretending that external provider
execution is part of the model's internal chain.

Phoenix spans contain metadata, timings, statuses, bounded error summaries,
provider run IDs, candidate counts, and MinIO object references. They never
contain API keys, raw video, unrestricted page contents, or full provider
responses.

## Asset storage

An `AssetStore` interface abstracts private S3-compatible object storage. The
initial implementation targets MinIO. Objects use job-scoped keys such as:

```text
social-analysis/{analysis_job_id}/source/image-1.jpg
social-analysis/{analysis_job_id}/source/video-1.mp4
social-analysis/{analysis_job_id}/scraper/response.json
```

The backend records durable artifact metadata—job ID, object key, kind, MIME
type, byte size, SHA-256, source/provider, and retention class—in a dedicated
analysis artifact table. Phoenix records the object key and checksum, not the
binary itself. Final marker image storage remains unchanged; MinIO is for
source and processing artifacts.

Temporary artifacts receive a lifecycle/retention class so cleanup can remove
large source media without deleting marker images. The bucket remains private;
the API does not expose arbitrary object paths.

## Phoenix configuration

Non-secret configuration belongs in a `[phoenix]` TOML section:

```toml
[phoenix]
enable = true
endpoint = "http://phoenix.home"
project = "roroadmap-social-analysis"
apikey = ""
apikeyenv = "PHOENIX_API_KEY"
```

`PHOENIX_API_KEY` is stored in Infisical per environment. Development and
production use their own Infisical environment and may use different Phoenix
projects or endpoints. The backend uses OTLP HTTP and gracefully disables
export when tracing is disabled or unavailable; analysis processing must not
fail because Phoenix is down.

## MinIO configuration

MinIO endpoint, bucket, region, and credentials are deployment configuration.
Credentials are resolved through Infisical with TOML/environment fallbacks,
using separate development and production values. Object upload failures are
handled according to artifact criticality: optional debug snapshots may be
skipped, while required media needed by a configured analyzer returns a clear
job error.

## Trace conventions

Every backend span includes:

- `analysis.job_id`
- `analysis.source_kind`
- `provider.name`
- `provider.operation`
- `provider.run_id` when available
- `result.status`
- `duration_ms`

Provider-specific fields are bounded and redacted. A span event records stage
transitions and summarized failures. LLM token and model data remains owned by
the wcodex-runner trace.

## Failure handling

- Phoenix unavailable: log a local warning and continue the analysis.
- MinIO unavailable for optional artifacts: continue and record the omission.
- MinIO unavailable for required analysis media: fail with a typed job error.
- Provider errors: keep existing provider fallback behavior and mark the span
  as error without leaking credentials.
- Trace/export queues are bounded so observability cannot exhaust worker
  memory.

## Testing

Unit tests cover trace/span construction, redaction, correlation metadata,
disabled-export behavior, MinIO key generation, checksums, upload failures,
and artifact persistence. Provider tests assert that Apify, Google Places,
and future Jev calls emit the expected operation metadata. An integration test
uses an HTTP test OTLP endpoint and an S3-compatible test server to verify the
end-to-end correlation without requiring production Phoenix or MinIO.

## Alternatives considered

1. Put all provider calls inside the wcodex trace. Rejected because it hides
   backend costs and makes external services look like model reasoning steps.
2. Use Phoenix as the binary/raw-response store. Rejected because traces are
   better for metadata and diagnosis, while MinIO is designed for large
   artifacts and retention policies.
3. Store only application logs. Rejected because logs do not provide the
   parent/child timing and correlation needed to compare Agent 1, Agent 2,
   Apify, Google Places, and Jev.
