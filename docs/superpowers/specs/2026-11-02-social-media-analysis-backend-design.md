# Social Media Analysis Backend Design

Date: 2026-11-02
Status: Proposed

## Goal

Replace the current n8n social-media analysis graph with a backend-owned,
asynchronous analysis service. The first client is a direct API request carrying
a URL. Later clients such as shortcuts, Telegram, or other integrations should
reuse the same analysis service rather than recreate its workflow.

## Scope

The first release accepts a public HTTP(S) URL and returns a tracked analysis
job. Instagram and Threads use dedicated scrapers when available. Other public
URLs use an agent-assisted fetch and extraction fallback.

The release includes:

- URL validation and safe remote fetching.
- Social-media extraction and normalized post data.
- OpenAI analysis for text and images.
- Video preprocessing into representative frames and optional audio
  transcription, followed by OpenAI analysis.
- OpenAI agent reasoning for description interpretation, comparison, and
  optional web research through wcodex-runner.
- Backend-owned geocoding/place lookup, nearby-marker lookup, validation, and
  marker creation.
- An application-level job status and structured final result.
- Infisical-backed provider secrets.

The release does not include Jev, Telegram-specific interaction, shortcut
integration, or direct replacement of every existing n8n behavior. Those are
follow-up clients or policy improvements after the core pipeline is stable.

## Architecture

```text
Client
  -> POST /social-media/analyses {url}
  -> RoroadMap backend creates analysis job
  -> scrape and normalize source
  -> analyze image or preprocess/analyze video
  -> submit reasoning task to wcodex-runner
  -> agent uses backend tools when needed:
       geocode/place lookup, nearby markers, web research
  -> backend validates structured result
  -> ask for confirmation or create marker
  -> client polls analysis status
```

The backend owns orchestration and side effects. The agent supplies structured
interpretation and recommendations; it does not receive database credentials,
create markers directly, or decide which secrets/capabilities it may use.

## Pipeline

### 1. Request and job creation

`POST /api/v1/social-media/analyses` accepts:

```json
{
  "url": "https://example.com/post"
}
```

The endpoint authenticates the caller, validates the URL, creates an analysis
record, and returns `202 Accepted` with a stable job ID. The initial status is
`queued`.

`GET /api/v1/social-media/analyses/{id}` returns status, progress, errors, and
the structured result when available. The API must not expose provider keys,
runner prompts, internal execution traces, or raw secret-bearing payloads.

### 2. Source acquisition

The backend first selects a source adapter:

- Instagram/Threads: dedicated scraper adapter.
- Other URLs: safe generic fetch or agent-assisted extraction fallback.

Remote fetching must allow only HTTP(S), block localhost/private-network and
metadata endpoints, enforce connection/read/response-size limits, and preserve
the canonical/final URL for auditability. A failed generic fetch produces a
structured `source_unavailable` result rather than an unbounded retry loop.

### 3. Normalization

Adapters produce a provider-independent post model containing source URL,
author, caption/text, publication metadata, media references, media type, and
fetch diagnostics. Raw provider payloads may be retained for debugging with
appropriate data-retention controls, but downstream components consume the
normalized model.

Known media type is parser output, not an LLM or rules-engine decision.

### 4. Media analysis

Static images are sent to an OpenAI vision-capable model with structured output.

Videos are downloaded under bounded resource limits, sampled into
representative frames, and optionally transcribed. Frames, transcript, and
video metadata are sent for OpenAI analysis. The media-analysis contract must
include confidence and a list of extracted facts, not only prose.

The implementation should retain a provider boundary so Gemini can remain an
optional fallback if testing shows that OpenAI video processing is insufficient.

### 5. Agent reasoning

The backend submits a bounded wcodex-runner job containing the normalized post,
media-analysis facts, and relevant existing-marker candidates. The job uses a
strict output schema with fields for extracted description, candidate location,
comparison result, evidence, confidence, requested follow-up, and final
recommendation.

The agent may request backend-owned tools for:

- Geocoding and place lookup after a candidate location is extracted.
- Nearby-marker lookup and candidate comparison.
- Web research when existing data is insufficient or contradictory.

Map lookup is not another agent. It is a deterministic backend service invoked
after location clues exist and before final duplicate/comparison validation.
Static map preview generation happens after a candidate is selected and is
needed for confirmation or presentation.

### 6. Final validation and side effects

The backend validates the agent result against the output schema and business
constraints. It must reject malformed, low-confidence, or contradictory
results rather than allowing the agent to create a marker directly.

Possible terminal/application states include:

- `completed`: analysis succeeded without marker creation.
- `awaiting_confirmation`: user approval is required.
- `created`: marker creation succeeded.
- `needs_input`: source or location information is insufficient.
- `failed`: processing failed after bounded retries.
- `cancelled`: explicitly cancelled by the caller.

Marker creation is idempotent and requires a backend authorization decision.

## wcodex-runner integration

The backend uses the runner as an asynchronous agent execution primitive, not
as the application job database. Each submission includes a canonical
application and operation context, a bounded timeout, and an output schema.

The backend persists the runner job ID and reconciles queued, running, done,
failed, cancelled, and timed-out states. It must check runner status before
retrying an uncertain submission. Runner logs remain diagnostic; structured
output is the success criterion.

Runner capabilities are selected server-side per operation. No user-provided
request may grant secrets, notifications, database access, or arbitrary tools.

## Secrets and Infisical

Scraper API keys and provider credentials move to Infisical. The migration
should inventory existing n8n and backend secrets and move them into narrowly
scoped environments/folders. Secrets must not appear in source code, n8n
prompts, runner prompts, tags, metadata, logs, or API responses.

The backend receives only the provider secrets required for its configured
operations. A runner job receives a secret capability only when the agent
itself must use that secret; otherwise provider calls remain behind backend
tools.

## Why Jev is deferred

The first implementation should use typed backend policy functions for routing
and validation. The current pipeline still requires LLM calls for description
interpretation, media understanding, and ambiguous comparisons, so adding Jev
would not remove the main complexity.

Jev can be introduced later if policies become runtime-editable, shared across
multiple clients, independently audited, or frequently changed by non-Go
developers. Its eventual boundary should be structured facts in and a typed
route/decision out; it should not own scraping, network calls, credentials, or
marker side effects.

## Testing and rollout

The migration should begin with fixture-based tests from representative
Instagram, Threads, video, image, generic-public-URL, inaccessible-URL, and
ambiguous-location cases. Compare structured results against the current n8n
workflow before enabling marker creation.

Rollout stages:

1. Shadow mode: run the backend pipeline without external side effects.
2. Result comparison: compare backend output with selected n8n executions.
3. Controlled marker creation for explicit test requests.
4. Promote the backend endpoint as the source of truth.
5. Retire the n8n workflow after a defined observation period.

Shortcuts, Telegram, and other clients should be added only after they can
submit to and consume this shared analysis-job contract.
