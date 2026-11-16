# Social Media Analysis Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an authenticated asynchronous RoroadMap API that accepts a public URL, analyzes its content with OpenAI and wcodex-runner, and returns a validated marker-analysis result without depending on the n8n graph.

**Architecture:** The Go backend owns URL safety, source adapters, normalization, media preprocessing, job persistence, runner reconciliation, map/marker tools, and final side effects. wcodex-runner performs bounded OpenAI-agent reasoning and returns strict structured output. Instagram/Threads receive dedicated adapters; other public URLs use a controlled generic-fetch fallback. Jev is intentionally not part of this first implementation.

**Tech Stack:** Go, Chi, GORM, existing RoroadMap integration authentication, existing external API audit, OpenAI-compatible API/agent execution through wcodex-runner, Infisical-managed secrets, existing TomTom/map and marker services.

## Global Constraints

- Accept only `http` and `https` URLs; block localhost, private-network, metadata-service, filesystem, and other non-public targets.
- The API returns `202 Accepted` for accepted jobs and exposes status through a separate GET endpoint.
- The backend validates all agent output before any marker side effect.
- Runner jobs use server-selected capabilities, canonical application/operation context, bounded timeouts, and strict output schemas.
- Secrets must not appear in source code, prompts, runner metadata, logs, or API responses.
- Static media type is parser output; neither Jev nor an LLM decides whether a post contains an image or video.
- OpenAI handles image analysis; video uses bounded frame/audio preprocessing with a provider boundary that permits a later fallback.
- Existing n8n behavior remains available during shadow and comparison rollout.

---

## File map

Create focused files rather than extending the already large `service/api_key_http.go`:

- `database/dbmodel/social_media_analysis_job.go`: persisted job state, normalized status constants, and JSON fields.
- `service/social_media_analysis_types.go`: request/response DTOs, normalized source/media models, agent result schema, and internal interfaces.
- `service/social_media_analysis_http.go`: authenticated POST/GET handlers and response serialization.
- `service/social_media_analysis_service.go`: orchestration state machine and final-result validation.
- `service/social_media_analysis_source.go`: URL validation, adapter selection, generic fetch limits, and normalized source acquisition.
- `service/social_media_analysis_media.go`: image dispatch, video frame/audio preprocessing boundaries, and media-analysis result handling.
- `service/wcodex_runner_client.go`: configurable runner HTTP client, submit/status contracts, bounded polling, and error normalization.
- `service/social_media_analysis_worker.go`: persisted-job claiming, runner reconciliation, retries, and startup worker.
- `service/social_media_analysis_tools.go`: backend-owned geocoding, nearby-marker, web-research request, and marker-preview tool boundaries.
- `service/social_media_analysis_*_test.go`: unit and HTTP tests grouped by boundary.
- `config/setting.go` and `config.example.toml`: runner, source-fetch, and media-processing settings; secret values remain environment/Infisical supplied.
- `database/dbmodel/base.go`: register the job model with existing AutoMigrate.
- `server.go`: register the authenticated analysis routes and start the worker.
- `scripts/migrations/`: add a reviewed migration only if deployment policy requires SQL migrations in addition to AutoMigrate.

Existing services to reuse rather than duplicate:

- Integration authentication and scope checks in `service/api_key_http.go`.
- Marker creation and nearby lookup handlers/services.
- TomTom geocoding and static-map services.
- External API audit helpers.
- Existing persisted-worker patterns in `service/calendar_sync_job_queue.go` and `service/calendar_sync_worker.go`.

## Task 1: Define the persisted analysis job contract

**Files:**
- Create: `database/dbmodel/social_media_analysis_job.go`
- Modify: `database/dbmodel/base.go`
- Create: `service/social_media_analysis_types.go`
- Test: `database/dbmodel/social_media_analysis_job_test.go`

**Interfaces:**
- `SocialMediaAnalysisJob` stores `JobID`, `SourceURL`, `Status`, `Stage`, `RunnerJobID`, `ResultJSON`, `ErrorCode`, `ErrorDetail`, `Attempts`, `MaxAttempts`, `NextAttemptAt`, `StartedAt`, and `CompletedAt`.
- Status constants are `queued`, `processing`, `awaiting_confirmation`, `completed`, `created`, `needs_input`, `failed`, and `cancelled`.
- `SocialMediaAnalysisRequest` contains only `URL string` for phase one.
- `SocialMediaAnalysisResponse` exposes job ID, status, stage, timestamps, error code, and typed result; it never exposes prompts or secrets.

- [ ] **Step 1: Write model and serialization tests**

Test that a new job defaults to `queued`, that status constants are stable, and that JSON serialization omits internal fields such as raw payloads and secret-bearing diagnostics.

- [ ] **Step 2: Run the focused tests and verify the missing model fails**

Run: `go test ./database/dbmodel ./service -run SocialMediaAnalysis -count=1`

Expected: FAIL because the model and DTOs do not exist yet.

- [ ] **Step 3: Implement the model and DTOs**

Use the existing GORM model conventions and add the model to `AutoMigration()`. Store normalized source/result payloads as bounded JSON text and add indexes for status, next-attempt time, and runner job ID.

- [ ] **Step 4: Run focused tests**

Run: `go test ./database/dbmodel ./service -run SocialMediaAnalysis -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add database/dbmodel/social_media_analysis_job.go database/dbmodel/base.go service/social_media_analysis_types.go database/dbmodel/social_media_analysis_job_test.go
git commit -m "feat: define social media analysis job contract"
```

## Task 2: Add safe URL validation and source adapters

**Files:**
- Create: `service/social_media_analysis_source.go`
- Create: `service/social_media_analysis_source_test.go`
- Modify: `config/setting.go`
- Modify: `config.example.toml`

**Interfaces:**
- `ValidatePublicSourceURL(rawURL string) (*url.URL, error)` rejects unsupported schemes, credentials, localhost, loopback, link-local, private, multicast, unspecified, and cloud metadata addresses.
- `SourceAdapter.Fetch(ctx context.Context, sourceURL *url.URL) (NormalizedSocialPost, error)` is the adapter boundary.
- `SelectSourceAdapter(sourceURL *url.URL) SourceAdapter` selects Instagram/Threads adapters first and a bounded generic adapter otherwise.

- [ ] **Step 1: Write failing URL-safety tests**

Cover valid public HTTPS, HTTP if enabled, missing scheme, `file://`, `ftp://`, localhost, `127.0.0.1`, private IPv4, IPv6 loopback, embedded credentials, redirect-to-private-address, oversized response, and timeout cases.

- [ ] **Step 2: Run the tests and verify failure**

Run: `go test ./service -run 'Test(ValidatePublicSourceURL|GenericSourceFetch)' -count=1`

Expected: FAIL because validation and adapters are absent.

- [ ] **Step 3: Implement validation and bounded generic fetching**

Resolve every redirect before accepting it, enforce configured connect/request/response limits, cap response bytes, preserve the final URL, and return typed errors such as `invalid_source_url`, `blocked_source_url`, `source_timeout`, and `source_too_large`.

- [ ] **Step 4: Implement normalized Instagram/Threads adapter interfaces**

Keep provider-specific parsing behind adapters. The first pass may wrap existing scraper behavior or return an explicit unavailable error where the repository has no reusable implementation; the service contract must not expose provider-specific payloads downstream.

- [ ] **Step 5: Run focused tests**

Run: `go test ./service -run 'Test(ValidatePublicSourceURL|GenericSourceFetch|SourceAdapter)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/social_media_analysis_source.go service/social_media_analysis_source_test.go config/setting.go config.example.toml
git commit -m "feat: add safe social media source acquisition"
```

## Task 3: Add media-analysis provider boundaries

**Files:**
- Create: `service/social_media_analysis_media.go`
- Create: `service/social_media_analysis_media_test.go`
- Modify: `config/setting.go`
- Modify: `config.example.toml`

**Interfaces:**
- `MediaAnalyzer.AnalyzeImage(ctx context.Context, input ImageAnalysisInput) (MediaAnalysisResult, error)`.
- `VideoPreprocessor.Prepare(ctx context.Context, input VideoInput) (PreparedVideo, error)`.
- `MediaAnalyzer.AnalyzeVideo(ctx context.Context, input PreparedVideo) (MediaAnalysisResult, error)`.
- `MediaAnalysisResult` contains extracted facts, confidence, media diagnostics, and provider metadata without credentials.

- [ ] **Step 1: Write failing tests for media routing and limits**

Test image dispatch, video dispatch, no-media behavior, maximum frame count, maximum video duration/bytes, and provider errors. Use fakes for OpenAI and the video preprocessor; do not call external APIs in unit tests.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./service -run 'Test(Media|Video)' -count=1`

Expected: FAIL because the provider boundaries are absent.

- [ ] **Step 3: Implement the media contracts and bounded preprocessing**

Keep actual OpenAI request construction behind an injectable provider. Preprocess videos into a configured number of representative frames and optional audio transcript input. Preserve a fallback interface for a later alternate provider without making Gemini part of the first path.

- [ ] **Step 4: Add structured-output validation**

Reject missing confidence/facts and malformed provider responses before they enter agent context.

- [ ] **Step 5: Run focused tests**

Run: `go test ./service -run 'Test(Media|Video)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/social_media_analysis_media.go service/social_media_analysis_media_test.go config/setting.go config.example.toml
git commit -m "feat: add openai media analysis boundaries"
```

## Task 4: Add the wcodex-runner client and structured agent contract

**Files:**
- Create: `service/wcodex_runner_client.go`
- Create: `service/wcodex_runner_client_test.go`
- Modify: `config/setting.go`
- Modify: `config.example.toml`

**Interfaces:**
- `RunnerClient.Submit(ctx context.Context, request RunnerSubmitRequest) (RunnerJob, error)`.
- `RunnerClient.Get(ctx context.Context, runnerJobID string) (RunnerJob, error)`.
- `RunnerJob` distinguishes queued, running, done, failed, cancelled, and timed-out states.
- `AgentAnalysisInput` contains normalized post data, media facts, existing-marker candidates, and explicit tool policy.
- `AgentAnalysisResult` contains recommendation, candidate location, comparison, evidence URLs, confidence, and follow-up action.

- [ ] **Step 1: Write HTTP contract tests with an in-memory test server**

Verify POST body includes canonical application/operation context, bounded timeout, strict output schema, and no credentials. Verify GET status decoding, non-2xx errors, malformed JSON, and done/failed/cancelled states.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./service -run 'TestRunnerClient' -count=1`

Expected: FAIL because the client is absent.

- [ ] **Step 3: Implement the client**

Use configurable `WCODEX_RUNNER_URL`, request timeout, and polling backoff. Do not automatically retry an unknown POST outcome until the original job status has been checked.

- [ ] **Step 4: Validate agent output**

Require a known recommendation, confidence bounds, evidence shape, and safe follow-up state. Keep web research as an agent tool/result, not as an implicit network side effect in the backend.

- [ ] **Step 5: Run focused tests**

Run: `go test ./service -run 'TestRunnerClient' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/wcodex_runner_client.go service/wcodex_runner_client_test.go config/setting.go config.example.toml
git commit -m "feat: integrate wcodex runner for analysis jobs"
```

## Task 5: Add backend-owned analysis tools

**Files:**
- Create: `service/social_media_analysis_tools.go`
- Create: `service/social_media_analysis_tools_test.go`

**Interfaces:**
- `GeocodeCandidate(ctx context.Context, candidate LocationCandidate) (LocationResolution, error)`.
- `FindNearbyMarkerCandidates(ctx context.Context, location LocationResolution) ([]MarkerCandidate, error)`.
- `BuildAnalysisToolContext(ctx context.Context, input NormalizedSocialPost, media MediaAnalysisResult) (ToolContext, error)`.

- [ ] **Step 1: Write tests for tool ordering and provider errors**

Verify geocoding occurs only after location clues exist, nearby lookup receives normalized coordinates, static-map preview is not generated before candidate selection, and external failures become typed degraded results.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./service -run 'TestSocialMediaAnalysisTools' -count=1`

Expected: FAIL because the analysis tool boundary is absent.

- [ ] **Step 3: Implement adapters over existing services**

Reuse TomTom geocoding, existing nearby marker query logic, marker-type lookup, and static preview functions. Do not duplicate marker creation or direct database query semantics from existing integration handlers.

- [ ] **Step 4: Run focused tests**

Run: `go test ./service -run 'TestSocialMediaAnalysisTools' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/social_media_analysis_tools.go service/social_media_analysis_tools_test.go
git commit -m "feat: expose backend analysis tools"
```

## Task 6: Implement persisted orchestration and worker reconciliation

**Files:**
- Create: `service/social_media_analysis_service.go`
- Create: `service/social_media_analysis_worker.go`
- Create: `service/social_media_analysis_service_test.go`
- Create: `service/social_media_analysis_worker_test.go`
- Modify: `server.go`

**Interfaces:**
- `CreateSocialMediaAnalysis(ctx context.Context, request SocialMediaAnalysisRequest, actor AnalysisActor) (*SocialMediaAnalysisJob, error)`.
- `ProcessSocialMediaAnalysisJob(ctx context.Context, jobID string) error`.
- `ReconcileSocialMediaAnalysisJobs(ctx context.Context, limit int) error`.
- `StartSocialMediaAnalysisWorker()`.

- [ ] **Step 1: Write state-machine tests**

Cover queued-to-processing, source failure, media failure, runner submission, runner success, runner failure, needs-input, awaiting-confirmation, marker creation, idempotent repeated processing, cancellation, and max-attempt exhaustion.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./service -run 'TestSocialMediaAnalysis(Job|Worker|Process)' -count=1`

Expected: FAIL because orchestration is absent.

- [ ] **Step 3: Implement job creation and processing**

Persist the job before doing external work. Process source acquisition and media analysis first, submit the runner job with normalized data, persist the runner ID, and reconcile runner completion before final validation. Store only bounded normalized/result JSON.

- [ ] **Step 4: Implement worker startup and claiming**

Follow the existing calendar worker pattern with database locking, bounded batches, retry backoff, and structured logs. Register the worker in `main()` after database initialization.

- [ ] **Step 5: Run focused tests**

Run: `go test ./service -run 'TestSocialMediaAnalysis(Job|Worker|Process)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/social_media_analysis_service.go service/social_media_analysis_worker.go service/social_media_analysis_service_test.go service/social_media_analysis_worker_test.go server.go
git commit -m "feat: process social media analysis jobs"
```

## Task 7: Add authenticated POST/GET API endpoints

**Files:**
- Create: `service/social_media_analysis_http.go`
- Create: `service/social_media_analysis_http_test.go`
- Modify: `server.go`

**Interfaces:**
- `CreateSocialMediaAnalysisHandler(w http.ResponseWriter, r *http.Request)` at `POST /api/v1/social-media/analyses`.
- `GetSocialMediaAnalysisHandler(w http.ResponseWriter, r *http.Request)` at `GET /api/v1/social-media/analyses/{id}`.

- [ ] **Step 1: Write HTTP tests**

Cover missing URL, malformed JSON, unsupported/unsafe URL, unauthorized caller, successful `202`, unknown job ID, caller access isolation, queued response, and completed structured result response.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./service -run 'TestSocialMediaAnalysisHTTP' -count=1`

Expected: FAIL because routes and handlers are absent.

- [ ] **Step 3: Implement handlers using existing integration authentication**

Add a dedicated API-key scope for analysis submission/status, or reuse an existing integration scope only if the existing permission model explicitly covers this operation. Return `202` for accepted jobs and consistent existing integration error envelopes.

- [ ] **Step 4: Register routes**

Register the routes in the existing authenticated integration router without adding logic to `api_key_http.go`.

- [ ] **Step 5: Run focused tests**

Run: `go test ./service -run 'TestSocialMediaAnalysisHTTP' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/social_media_analysis_http.go service/social_media_analysis_http_test.go server.go
git commit -m "feat: expose social media analysis API"
```

## Task 8: Configure Infisical-backed secrets and runner deployment settings

**Files:**
- Modify: `config/setting.go`
- Modify: `config.example.toml`
- Modify: `RM-Map-Backend/README.md`
- Create: `docs/superpowers/specs/2026-11-02-social-media-secret-migration.md` only if the inventory reveals a separate migration scope.

**Interfaces:**
- Runtime configuration supplies non-secret runner URL, timeouts, provider selection, and limits.
- Infisical supplies scraper/provider secret values through the deployment environment or approved secret-injection mechanism.

- [ ] **Step 1: Write configuration tests**

Verify missing runner URL disables or clearly fails the feature, non-positive limits receive safe defaults, and secret values are not included in config serialization or diagnostics.

- [ ] **Step 2: Implement configuration wiring and documentation**

Document required Infisical folders/keys, environment naming, rotation expectations, and the distinction between backend secrets and runner capabilities. Do not commit secret values.

- [ ] **Step 3: Run configuration tests**

Run: `go test ./config ./service -run 'Test(SocialMedia|Runner)' -count=1`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add config/setting.go config.example.toml README.md
git commit -m "chore: configure social media analysis secrets"
```

## Task 9: Add fixture, integration, and shadow-rollout verification

**Files:**
- Create: `service/social_media_analysis_fixture_test.go`
- Modify: `README.md` or `docs/` with local verification instructions.
- Modify: existing smoke-test scripts only after the endpoint contract is stable.

- [ ] **Step 1: Add fixtures for representative inputs**

Include public URL normalization, Instagram/Threads adapter output, image analysis output, video frame/transcript output, inaccessible URL, ambiguous location, duplicate candidate, web-research-required, and malformed-agent-output cases. Use sanitized payloads without provider secrets or personal tokens.

- [ ] **Step 2: Add an end-to-end fake-runner test**

Run the complete API-to-job-to-result flow against fake scraper, media, runner, geocoder, and marker services. Assert that marker creation is not reached in shadow mode and that the final result is structured.

- [ ] **Step 3: Run the backend verification suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 4: Run static checks**

Run: `go vet ./...`

Expected: PASS with no new warnings.

- [ ] **Step 5: Document shadow-mode rollout**

Record how to run the backend pipeline without marker side effects, compare selected outputs with n8n, enable controlled creation, and monitor runner/job failure categories.

- [ ] **Step 6: Commit**

```bash
git add service/social_media_analysis_fixture_test.go README.md docs
git commit -m "test: verify social media analysis rollout"
```

## Execution notes

Implement tasks in order. Each task must pass its focused tests before the
next task begins. Do not remove or disable the n8n workflow until shadow-mode
comparison and controlled marker-creation verification are complete.
