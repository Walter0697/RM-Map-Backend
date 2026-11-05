# Social Analysis Observability and Asset Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add non-blocking Phoenix tracing and private MinIO artifact storage to the social-media analysis pipeline, with Infisical/TOML configuration and correlation metadata.

**Architecture:** The backend creates short-lived OpenTelemetry spans for each synchronous analysis stage and exports them to a dedicated Phoenix project. wcodex-runner remains in its existing Phoenix project; backend spans correlate to runner jobs through analysis and runner IDs. A bounded S3-compatible asset store saves source snapshots and selected media, while a database artifact record and Phoenix attributes retain only references and checksums.

**Tech Stack:** Go 1.22, OpenTelemetry Go SDK with OTLP/HTTP exporter, MinIO Go SDK (`minio-go/v7`), GORM/Postgres, Infisical-backed `SecretResolver`, Phoenix, existing social-media worker and wcodex client.

## Global Constraints

- Never log or store API keys, Infisical credentials, raw authorization headers, or unrestricted provider payloads.
- Phoenix export is best-effort and must never make analysis processing fail.
- Asset uploads are bounded by configured size and timeout limits.
- Development and production use separate Infisical environments and may use separate Phoenix projects and MinIO buckets.
- Preserve existing Apify, Google Places, wcodex, marker, and image fallback behavior when observability is disabled or unavailable.
- Every implementation task follows red-green testing and ends with a focused test command.

---

### Task 1: Add Phoenix and MinIO configuration and secret resolution

**Files:**
- Modify: `config/setting.go`
- Modify: `config/auth.go`
- Modify: `config.example.toml`
- Modify: `service/secret_resolver.go`
- Test: `config/setting_test.go`
- Test: `service/secret_resolver_test.go`

**Interfaces:**
- Produce `config.PhoenixSetting` with `Enable`, `Endpoint`, `Project`, `APIKey`, `APIKeyEnv`, and bounded export timeout/batch settings.
- Produce `config.MinIOSetting` with `Enable`, `Endpoint`, `Region`, `Bucket`, `AccessKey`, `AccessKeyEnv`, `SecretKey`, `SecretKeyEnv`, and bounded upload settings.
- Resolve `PHOENIX_API_KEY`, `MINIO_ACCESS_KEY`, and `MINIO_SECRET_KEY` through Infisical, configured environment names, and TOML fallback in that order.

- [ ] Write tests proving defaults, environment overrides, and TOML fallback for Phoenix and MinIO settings.
- [ ] Run `GOCACHE=/tmp/roroadmap-go-cache go test ./config ./service -run 'Test.*(Phoenix|MinIO|SecretResolver)' -count=1` and verify the new tests fail before implementation.
- [ ] Add the settings structs, defaults, environment overrides, example TOML sections, and secret-name mappings without returning secret values in errors.
- [ ] Run the focused test command again and verify it passes.
- [ ] Commit with `git commit -m "feat: configure social analysis observability"`.

### Task 2: Add the Phoenix tracing adapter

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `service/observability.go`
- Test: `service/observability_test.go`

**Interfaces:**
- `NewSocialAnalysisTracer(setting config.PhoenixSetting, secrets SecretResolver) (*SocialAnalysisTracer, error)`.
- `(*SocialAnalysisTracer).Start(ctx context.Context, name string, attributes map[string]string) (context.Context, SpanHandle)`.
- `SpanHandle` supports `SetAttribute`, `AddEvent`, `RecordError`, `SetStatus`, and `End`.
- A no-op tracer is returned when Phoenix is disabled or unavailable.

- [ ] Add OpenTelemetry SDK, OTLP/HTTP exporter, and SDK dependencies with Go 1.22-compatible versions using `go get`, then run `go mod tidy`.
- [ ] Write tests using an in-process OTLP HTTP test receiver to assert project name, provider attributes, job correlation IDs, and error status; add a disabled-export test.
- [ ] Run `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestSocialAnalysisTracer' -count=1` and verify failure before implementation.
- [ ] Implement bounded batch export, explicit endpoint handling, API-key authorization through the resolver, redaction of secret-like attributes, and safe shutdown/flush behavior.
- [ ] Run the focused tracing tests and verify that exporter failures do not propagate to callers.
- [ ] Commit with `git commit -m "feat: add Phoenix tracing adapter"`.

### Task 3: Add MinIO asset storage and artifact records

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `service/asset_store.go`
- Create: `service/minio_asset_store.go`
- Create: `service/asset_store_test.go`
- Create: `database/dbmodel/social_media_analysis_artifact.go`
- Modify: `database/dbmodel/base.go`
- Test: `database/dbmodel/social_media_analysis_artifact_test.go`

**Interfaces:**
- `AssetStore.Put(ctx context.Context, key string, content io.Reader, size int64, contentType string) (StoredAsset, error)`.
- `StoredAsset` contains only bucket, key, content type, byte size, checksum, and retention class.
- `buildSocialMediaAssetKey(jobID, kind string, index int, extension string) string` returns a path under `social-analysis/{jobID}/`.
- `SocialMediaAnalysisArtifact` stores job ID, object key, kind, source/provider, MIME type, byte size, SHA-256, and retention class.

- [ ] Write failing tests for deterministic key generation, checksum calculation, bounded reads, private MinIO client setup, and upload failure handling.
- [ ] Run `GOCACHE=/tmp/roroadmap-go-cache go test ./service ./database/dbmodel -run 'Test.*(Asset|Artifact)' -count=1` and verify failure before implementation.
- [ ] Add the MinIO client adapter using private bucket/object operations, configured timeouts, and no public URL generation.
- [ ] Add the artifact model to `AutoMigration()` and implement metadata persistence helpers.
- [ ] Run focused asset and model tests and verify they pass.
- [ ] Commit with `git commit -m "feat: store social analysis artifacts in MinIO"`.

### Task 4: Instrument source scraping and artifact capture

**Files:**
- Modify: `service/social_media_analysis_worker.go`
- Modify: `service/social_media_analysis_apify.go`
- Modify: `service/social_media_analysis_source.go`
- Test: `service/social_media_analysis_apify_test.go`
- Test: `service/social_media_analysis_worker_test.go`

**Interfaces:**
- Source spans use `provider.name` (`apify` or `generic`), `provider.operation` (`fetch`), analysis job ID, source kind, bounded duration, run ID when available, and result status.
- `captureSocialAnalysisArtifacts` uploads normalized source JSON and explicitly bounded media snapshots when asset storage is enabled, then persists artifact metadata.

- [ ] Add failing tests proving Apify and generic source spans include provider metadata and that a normalized source snapshot gets a job-scoped MinIO key.
- [ ] Run the focused source tests and verify failure before implementation.
- [ ] Inject the tracer and asset store into the worker/source adapter construction without changing the existing `SourceAdapter` contract.
- [ ] Record Apify actor/run IDs in span attributes and persist only bounded response metadata; never include the token in query/log/span fields.
- [ ] Capture normalized JSON and configured media artifacts with size/time limits; treat optional artifact failures as warnings while preserving the existing source-fetch result.
- [ ] Run the focused tests and verify they pass.
- [ ] Commit with `git commit -m "feat: trace social source scraping"`.

### Task 5: Instrument Google Places, Jev boundary, and wcodex submissions

**Files:**
- Modify: `service/google_places.go`
- Modify: `service/social_media_analysis_worker.go`
- Create: `service/jev.go`
- Test: `service/google_places_test.go`
- Test: `service/social_media_analysis_worker_test.go`
- Test: `service/jev_test.go`

**Interfaces:**
- Google spans use `provider.name=google_places`, `provider.operation=search`, query fingerprint, candidate count, status, and analysis job ID; the API key and raw query are excluded unless explicitly redacted.
- `JevClient` is an interface boundary with a no-op/unconfigured implementation; this task instruments the boundary but does not add Jev behavior.
- Runner submit spans use `provider.name=wcodex_runner`, operation, runner job ID, stage, and analysis job ID; the runner continues owning its detailed agent trace.

- [ ] Write failing tests for Google success/error span metadata, Jev no-op behavior, and separate Agent 1/Agent 2 runner operation attributes.
- [ ] Run focused tests and verify failure before implementation.
- [ ] Wrap provider calls and runner submissions in short-lived spans with correlation attributes and bounded events.
- [ ] Add the Jev interface and instrumentation seam without making Jev a required runtime dependency.
- [ ] Run focused tests and verify they pass, including the existing location-verification tests.
- [ ] Commit with `git commit -m "feat: trace social analysis providers"`.

### Task 6: Document deployment setup and verify the integrated pipeline

**Files:**
- Modify: `docs/social-media-analysis.md`
- Modify: `config.example.toml`
- Create: `docs/superpowers/verification/social-analysis-observability.md`

**Interfaces:**
- Documentation specifies development and production Infisical entries, Phoenix project names, MinIO bucket setup, retention expectations, and correlation fields.

- [ ] Document these Infisical secrets: `PHOENIX_API_KEY`, `MINIO_ACCESS_KEY`, and `MINIO_SECRET_KEY`, with separate `dev` and `prod` values under `/roroadmap`.
- [ ] Document non-secret endpoint/project/bucket settings and explain that the wcodex runner remains in project `wcodex` while backend provider spans use `roroadmap-social-analysis`.
- [ ] Add an integration verification checklist for one clear-match URL, one Agent 2 URL, a provider failure, Phoenix-disabled mode, and MinIO-disabled mode.
- [ ] Run `GOCACHE=/tmp/roroadmap-go-cache go test ./...`, `GOCACHE=/tmp/roroadmap-go-cache go vet ./...`, and `git diff --check`.
- [ ] Run a local test with Phoenix and MinIO configured, confirm the trace contains the expected provider spans, confirm artifacts are private and referenced by checksum, and record the result without recording credentials.
- [ ] Commit with `git commit -m "docs: document social analysis observability"`.

