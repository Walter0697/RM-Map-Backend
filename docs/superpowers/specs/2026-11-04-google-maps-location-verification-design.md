# Google Maps Location Verification

## Goal

Improve social-media marker creation by verifying agent-extracted locations with Google Places, while invoking a second agent only when the verification result is ambiguous.

## Scope

This change applies to the existing authenticated social-media analysis pipeline. It covers Google Places lookup, deterministic candidate comparison, conditional second-agent adjudication, configuration, and tests.

Jev is intentionally out of scope for this version. The design must leave a clear routing boundary where Jev can later replace or supplement deterministic triage.

## Current behavior

The source adapter scrapes and normalizes a public URL. The worker submits the normalized post and media references to wcodex-runner. The agent returns a structured analysis, and a marker is created when the recommendation is `create_marker` and a valid location is present.

## Proposed flow

```text
scrape source
    |
    v
Agent 1: analyze post and extract candidate location
    |
    +--> no location --------------------------> needs_input / no marker
    |
    v
Google Places Text Search (optional)
    |
    v
backend candidate comparison
    |
    +--> one strong match --------------------> create marker
    |
    +--> no match or conflicting matches ------> Agent 2
                                                    |
                                                    +--> confirmed match -> create marker
                                                    +--> uncertain       -> needs_input
```

Agent 1 remains the existing wcodex analysis call. It returns the post metadata, marker metadata, and a structured location candidate. Google is used as an evidence provider, not as the final decision-maker.

Agent 2 is submitted only when Google returns no usable result or multiple/conflicting results. Its prompt includes the normalized post, Agent 1's candidate, and the Google candidates. It must return the same validated analysis shape, including the final location and marker type.

When Google is disabled or its key is not configured, the existing Agent 1-only behavior remains active. This preserves local development and deployments that do not use Google Maps yet.

## Google Places integration

Use Places API (New) Text Search at:

```text
https://places.googleapis.com/v1/places:searchText
```

The backend sends a free-text `textQuery` derived from the agent's candidate name and address. The request uses a restricted field mask containing only the data needed for comparison and marker creation: place ID, display name, formatted address, location, types, and Google Maps URI.

The integration must:

- use a bounded HTTP timeout;
- cap the number of returned candidates;
- treat non-2xx responses and malformed responses as verification errors;
- never log the API key or include it in job metadata;
- expose an interface so tests can use a fake provider;
- preserve the agent-only path when the integration is disabled.

## Candidate comparison

The backend compares the agent candidate with Google results using deterministic signals:

- normalized name token overlap;
- normalized address token overlap;
- coordinate distance when both candidates include coordinates;
- exact place ID reuse when a later lookup is performed.

The comparison returns one of three outcomes:

- `match`: one candidate clearly dominates and passes the configured thresholds;
- `ambiguous`: multiple candidates remain plausible or evidence conflicts;
- `no_match`: no Google candidate is sufficiently related.

`match` permits marker creation without Agent 2. `ambiguous` and `no_match` route to Agent 2 when the agent has enough information to adjudicate. If Agent 2 cannot establish a reliable location, the job completes as `needs_input` without creating a marker.

Thresholds and comparison policy are backend-owned configuration, not agent instructions.

## Job state and recovery

The existing job record and runner job ID are reused. After Agent 1 completes:

- a clear Google match creates the marker immediately;
- an ambiguous verification submits Agent 2 and changes the job stage to a comparison-agent stage;
- the second runner job ID replaces the first runner job ID;
- Agent 2 receives Agent 1's full result in its prompt, so recovery does not depend on an in-memory result;
- the final validated result remains the job's `result_json`.

No new database table is required for this version. Verification details should be included in job result metadata only if the existing response contract can support them without exposing provider secrets.

## Configuration and secrets

Infisical is the preferred secret source:

```text
Environment: dev
Path: /roroadmap
Key: GOOGLE_MAPS_API_KEY
```

The API key must be restricted in Google Cloud to the Places API (New) and protected with quotas/billing controls.

Open-source deployments may use TOML fallback configuration:

```toml
[googlemaps]
enable=false
baseurl="https://places.googleapis.com"
timeoutms=5000
maxresults=5
```

Infisical takes precedence over explicitly configured environment values, which take precedence over TOML values, following the existing secret-resolution policy. The API key must never be committed or written to logs, prompts, or persisted job metadata.

## Testing

Tests must cover:

- Google Places request construction and field-mask headers;
- successful response parsing;
- non-2xx and malformed responses;
- name/address/coordinate match, ambiguity, and no-match outcomes;
- Agent 2 submission only for ambiguous/no-match cases;
- Google-disabled fallback to the existing Agent 1-only path;
- marker creation using the verified location;
- secret resolution precedence and missing-key behavior.

The live test requires a development Google Maps key in Infisical. Unit and integration tests must not require network access to Google.

## Non-goals

- Jev integration;
- replacing the existing source scrapers;
- Google Maps frontend rendering;
- automatic web search beyond Google Places;
- changing the authenticated POST/GET API contract.
