# Social Analysis Trace Thread Design

## Goal

Represent one social-media analysis job as one Phoenix trace, including work that occurs across worker polling cycles, while linking the separate wcodex-runner trace to the same analysis job.

## Design

Each `SocialMediaAnalysisJob` stores a W3C `traceparent` value. On the first worker pass, the backend starts an `analysis.run` span, persists its trace context, and creates child spans for source fetching, artifact capture, and Agent 1 submission. On later polling passes, the worker extracts the persisted trace context and starts a continuation span in the same trace before polling wcodex, running Google Places, submitting Agent 2, or creating the marker.

The existing provider spans remain best-effort and must not affect analysis behavior. The trace context is metadata only and is never exposed through the public analysis response. The database column is migrated through the existing GORM auto-migration path.

The wcodex runner remains in Phoenix project `wcodex`; the backend trace remains in `roroadmap-social-analysis`. Both sides continue to use `analysis.job_id` and `runner_job_id` for cross-project correlation. The runner request also carries the W3C `traceparent` in its context metadata when available so the runner can link its own trace.

## Trace shape

```text
analysis.run
├── source.fetch
├── artifact.upload
├── wcodex_runner.submit (agent_1)
├── google_places.search
├── wcodex_runner.submit (agent_2, only when comparison is ambiguous)
└── marker.create
```

## Error handling

- Missing or malformed trace context creates a new trace and does not fail the job.
- Phoenix export failures remain non-fatal.
- Existing job retry and runner polling behavior is unchanged.
- Traceparent values are internal metadata and are not logged as credentials or returned by HTTP handlers.

## Testing

- Test traceparent injection/extraction and malformed-context fallback.
- Test that a resumed job reuses the original trace ID.
- Test that source, provider, and runner spans receive the propagated context.
- Test that runner request context contains the analysis identifiers and traceparent without changing the public API response.
- Run the full Go test suite and `go vet`.
