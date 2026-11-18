# Google Places Phoenix Outcomes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record structured Google Places success and failure outcomes inside the existing social-analysis Phoenix trace while preserving agent-only fallback.

**Architecture:** Extend the Google Places provider's existing span with safe error classification derived from HTTP status and Google error payloads. Keep the provider call on the caller's context, and annotate the surrounding analysis flow when it falls back so all events remain grouped under `analysis.run`.

**Tech Stack:** Go, `net/http`, existing OpenTelemetry/Phoenix tracer helpers, Go tests.

## Global Constraints

- Never record Google API keys or raw location queries in telemetry.
- Preserve the current agent-only fallback when Google Places is unavailable.
- Keep all Google spans children of the caller-provided social-analysis context.
- Use the existing repository tracing conventions and avoid introducing a new tracing dependency.

---

### Task 1: Classify Google Places outcomes and trace the provider result

**Files:**
- Modify: `service/google_places.go`
- Test: `service/google_places_test.go`

**Interfaces:**
- Consumes: existing `GooglePlacesProvider.Search(ctx, query)` and provider span.
- Produces: stable span attributes for `google.places.outcome`, `google.places.http_status`, and safe Google error status when present.

- [ ] **Step 1: Write the failing tests**

  Add tests that make a local HTTP server return representative Google error payloads and assert the provider span records:

  ```go
  func TestGooglePlacesSearchClassifiesQuotaError(t *testing.T) {
      // Return HTTP 429 with a Google error status of RESOURCE_EXHAUSTED.
      // Assert outcome is "quota_or_rate_limit" and HTTP status is "429".
  }

  func TestGooglePlacesSearchClassifiesBillingError(t *testing.T) {
      // Return HTTP 403 with a Google error status of PERMISSION_DENIED.
      // Assert outcome is "billing_or_api_key".
  }

  func TestGooglePlacesSearchClassifiesGenericHTTPError(t *testing.T) {
      // Return HTTP 500 with a non-Google error body.
      // Assert outcome is "http_error".
  }
  ```

  Use the existing in-memory tracer test helper so assertions inspect real span attributes rather than only the mock HTTP call.

- [ ] **Step 2: Run the focused tests and verify they fail**

  Run:

  ```bash
  cd /home/git/RoroadMap/RM-Map-Backend
  GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestGooglePlacesSearchClassifies' -count=1
  ```

  Expected: FAIL because the new outcome attributes are not emitted yet.

- [ ] **Step 3: Implement minimal classification and attributes**

  Parse only the bounded Google error response needed for classification. Map `429`, `RESOURCE_EXHAUSTED`, `RATE_LIMIT_EXCEEDED`, `dailyLimitExceeded`, and `rateLimitExceeded` to `quota_or_rate_limit`; map permission-denied/billing/key-invalid statuses to `billing_or_api_key`; map remaining non-2xx responses to `http_error`. Set the provider span error status and safe attributes before returning the existing error shape.

- [ ] **Step 4: Run focused tests and verify they pass**

  Run the same focused command and expect all new classification tests to pass.

- [ ] **Step 5: Commit the provider tracing change**

  ```bash
  git add service/google_places.go service/google_places_test.go
  git commit -m "feat: classify Google Places failures in Phoenix"
  ```

### Task 2: Trace the Google fallback decision in the analysis run

**Files:**
- Modify: `service/social_media_analysis_worker.go`
- Test: `service/social_media_analysis_worker_test.go`

**Interfaces:**
- Consumes: the provider error returned by `maybeSubmitLocationComparison` and its existing analysis context.
- Produces: an `analysis.google_places_fallback` child span with safe outcome and fallback attributes, while returning the existing agent-only behavior.

- [ ] **Step 1: Write the failing test**

  Add a worker test with a fake Google provider returning a quota-classified error. Assert the function still keeps the first agent result and that the captured span contains `google.places.fallback=true` and the classified outcome.

- [ ] **Step 2: Run the focused test and verify it fails**

  ```bash
  GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestMaybeSubmitLocationComparison.*Fallback' -count=1
  ```

  Expected: FAIL because the fallback span/attributes are not present.

- [ ] **Step 3: Implement the fallback span**

  Start the child span from the context already passed to `maybeSubmitLocationComparison`, copy only the safe classification, mark the fallback outcome, preserve the existing log and return values, and end the span on every path.

- [ ] **Step 4: Run focused and package tests**

  ```bash
  GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestMaybeSubmitLocationComparison|TestGooglePlacesSearch' -count=1
  GOCACHE=/tmp/roroadmap-go-cache go test ./service -count=1
  ```

- [ ] **Step 5: Commit the analysis fallback tracing change**

  ```bash
  git add service/social_media_analysis_worker.go service/social_media_analysis_worker_test.go
  git commit -m "feat: trace Google Places fallback in analysis runs"
  ```

### Task 3: Verify the complete backend behavior

**Files:**
- No production file changes expected.

- [ ] **Step 1: Run formatting**

  ```bash
  gofmt -w service/google_places.go service/google_places_test.go service/social_media_analysis_worker.go service/social_media_analysis_worker_test.go
  ```

- [ ] **Step 2: Run the complete backend test suite**

  ```bash
  GOCACHE=/tmp/roroadmap-go-cache go test ./... -count=1
  ```

- [ ] **Step 3: Review the diff for telemetry safety**

  Confirm that API keys, authorization headers, and raw search text are absent from span attributes and error logs.

- [ ] **Step 4: Commit any formatting-only changes**

  ```bash
  git status --short
  ```

  If formatting changed tracked files, commit them with:

  ```bash
  git add service/google_places.go service/google_places_test.go service/social_media_analysis_worker.go service/social_media_analysis_worker_test.go
  git commit -m "chore: format Google Places tracing changes"
  ```
