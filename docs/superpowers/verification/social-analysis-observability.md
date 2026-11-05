# Social Analysis Observability Verification

## Configuration

- [ ] Infisical `dev` contains `PHOENIX_API_KEY`, `MINIO_ACCESS_KEY`, and
      `MINIO_SECRET_KEY` at `/roroadmap`.
- [ ] Development TOML enables `[phoenix].enable` and `[minio].enable` with
      the development Phoenix endpoint/project and MinIO endpoint/bucket.
- [ ] Production uses separate Infisical values and storage destinations.

## Clear-match analysis

- [ ] Submit a public post with an explicit venue.
- [ ] Confirm the backend trace in `roroadmap-social-analysis` includes
      `source.fetch`, `google_places.search` when enabled, and
      `wcodex_runner.submit` with `analysis.stage=agent_1`.
- [ ] Confirm those spans share one trace ID, including after at least one
      worker polling cycle; the analysis job stores an internal traceparent.
- [ ] Confirm the runner job appears in project `wcodex` and shares the
      analysis job ID, runner job ID, and traceparent correlation metadata.
- [ ] Confirm the marker is created and an artifact record contains checksum,
      size, MIME type, and a private MinIO object key.

## Agent 2 analysis

- [ ] Submit a post whose location produces ambiguous or no Google Places
      matches.
- [ ] Confirm a second `wcodex_runner.submit` span has
      `analysis.stage=agent_2` and operation
      `social-media-location-comparison`.
- [ ] Confirm Agent 2 remains connected to the same backend analysis trace.
- [ ] Confirm the final marker or needs-input result is persisted once.

## Failure and disabled paths

- [ ] Stop or make Phoenix unavailable; analysis still completes and logs a
      bounded exporter warning.
- [ ] Stop or make MinIO unavailable; optional artifact capture is skipped and
      the source/marker flow remains usable.
- [ ] Make Google Places fail; the existing Agent 1 fallback remains active.
- [ ] Verify traces and logs contain no API keys, authorization headers, or
      raw video payloads.

## Commands

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./...
GOCACHE=/tmp/roroadmap-go-cache go vet ./...
git diff --check
```
