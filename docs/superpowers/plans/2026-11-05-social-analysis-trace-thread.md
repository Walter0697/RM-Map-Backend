# Social Analysis Trace Thread Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist and propagate one OpenTelemetry trace context across every worker pass for a social-analysis job, while correlating wcodex runner jobs across Phoenix projects.

**Architecture:** Store a W3C `traceparent` on `SocialMediaAnalysisJob`. Each `ProcessSocialMediaAnalysisJob` invocation extracts that context, starts an `analysis.run` continuation span, and passes the resulting context to source, artifact, Google, runner, and marker operations. Add the traceparent to the runner context payload for cross-project correlation without changing the public API response.

**Tech Stack:** Go 1.22, OpenTelemetry SDK 1.24, OTLP HTTP, GORM/PostgreSQL, Phoenix, wcodex-runner.

## Global Constraints

- Trace propagation is best-effort metadata and must never fail analysis jobs.
- Secrets, API keys, prompts, raw content, and media bytes must not be added to trace attributes.
- Phoenix backend project remains `roroadmap-social-analysis`; wcodex remains `wcodex`.
- Existing public social-analysis response JSON must not expose trace metadata.
- Use W3C Trace Context and existing GORM auto-migration.

---

### Task 1: Add traceparent persistence and propagation helpers

**Files:**
- Modify: `database/dbmodel/social_media_analysis_job.go`
- Modify: `database/dbmodel/base.go` only if the existing migration registration needs adjustment
- Modify: `service/observability.go`
- Create: `service/observability_trace_context_test.go`

**Interfaces:**
- Produces `TraceParent string` on `dbmodel.SocialMediaAnalysisJob` with a bounded database column.
- Produces helpers `injectSocialAnalysisTraceParent(context.Context) string` and `extractSocialAnalysisTraceParent(context.Context, string) context.Context`.

- [ ] **Step 1: Write the failing tests**

Add tests that start a real `SocialAnalysisTracer` with a recording exporter, inject a traceparent from a started span, extract it into a new context, and assert the extracted span has the same trace ID. Add a malformed-traceparent test asserting extraction leaves the context usable without panic. Add a model test asserting `TraceParent` is part of the job model.

- [ ] **Step 2: Run the focused tests and verify they fail**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service ./database/dbmodel -run 'TraceParent|TraceContext' -count=1
```

Expected: compilation failure because the propagation helpers and model field do not exist.

- [ ] **Step 3: Implement the minimal propagation support**

Add the model field:

```go
TraceParent string `json:"-" gorm:"size:255"`
```

Use `go.opentelemetry.io/otel/propagation.TraceContext{}` with `propagation.MapCarrier` to inject and extract only the `traceparent` header. Return an unchanged context for blank or invalid values.

- [ ] **Step 4: Run the focused tests and verify they pass**

Run the same command. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add database/dbmodel/social_media_analysis_job.go service/observability.go service/observability_trace_context_test.go
git commit -m "feat: persist social analysis trace context"
```

### Task 2: Thread one trace through worker stages and retries

**Files:**
- Modify: `service/social_media_analysis_worker.go`
- Modify: `service/social_media_analysis_worker_test.go`
- Modify: `service/social_media_analysis_http.go` only if response mapping requires an explicit internal-field assertion

**Interfaces:**
- Consumes `TraceParent` and propagation helpers from Task 1.
- Produces a worker context in which `source.fetch`, artifact capture, runner submission, Google Places, and marker creation are descendants of `analysis.run`.

- [ ] **Step 1: Write failing worker tests**

Add a recording tracer test that processes a job twice: the first pass must persist a non-empty traceparent, and a second pass must start its `analysis.run` span with the same trace ID. Add assertions that the source fetch and runner submission receive the analysis context rather than the original root context. Add a JSON response test confirming `TraceParent` is not exposed.

- [ ] **Step 2: Run focused tests and verify failure**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Trace|SocialMediaAnalysis.*Response' -count=1
```

Expected: FAIL because the worker does not persist or extract trace context yet.

- [ ] **Step 3: Implement worker context threading**

At the beginning of `ProcessSocialMediaAnalysisJob`, extract `job.TraceParent`, start `analysis.run`, and inject/persist the resulting context when the job has no traceparent. Pass that context into `reconcileRunnerJob`, source fetch, artifact capture, and Agent 1 submission. Ensure the continuation span ends with the invocation. Pass the same context into Google Places and Agent 2 submission through the existing function parameters.

Do not add trace fields to `SocialMediaAnalysisResponse`.

- [ ] **Step 4: Run focused tests and verify green**

Run the focused command again, then:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -count=1
```

- [ ] **Step 5: Commit**

```bash
git add service/social_media_analysis_worker.go service/social_media_analysis_worker_test.go service/social_media_analysis_http.go
git commit -m "feat: thread social analysis traces across retries"
```

### Task 3: Correlate wcodex runner jobs

**Files:**
- Modify: `service/wcodex_runner_client.go`
- Modify: `service/wcodex_runner_client_test.go`
- Modify: `service/social_media_analysis_worker.go`

**Interfaces:**
- Extends `RunnerSubmitRequest` with internal `TraceParent`, `AnalysisJobID`, and `TraceAttributes` fields that are serialized only into the runner context payload.
- Keeps existing runner HTTP endpoints and public application/operation fields stable.

- [ ] **Step 1: Write failing request-payload tests**

Extend the runner client test server to assert that a submit payload contains `analysis_job_id`, `traceparent`, and the existing application/operation values when supplied. Assert absent optional fields remain absent for unrelated runner callers.

- [ ] **Step 2: Run focused test and verify failure**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'WcodexRunner|Runner' -count=1
```

Expected: FAIL because the payload does not yet carry the correlation fields.

- [ ] **Step 3: Implement correlation fields**

Add optional request fields and include them under the existing JSON `context` object only when non-empty. Populate them in both Agent 1 and Agent 2 calls from the current analysis job and trace context.

- [ ] **Step 4: Run focused tests and full service tests**

Run the focused command and:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -count=1
```

- [ ] **Step 5: Commit**

```bash
git add service/wcodex_runner_client.go service/wcodex_runner_client_test.go service/social_media_analysis_worker.go
git commit -m "feat: correlate wcodex jobs with analysis traces"
```

### Task 4: Verify, document, and push

**Files:**
- Modify: `docs/social-media-analysis.md`
- Modify: `docs/superpowers/verification/social-analysis-observability.md`

- [ ] **Step 1: Update documentation**

Document that one analysis job produces one backend trace across worker retries, while wcodex remains in its own Phoenix project and is correlated by `analysis.job_id`, `runner_job_id`, and traceparent context.

- [ ] **Step 2: Run complete verification**

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./...
GOCACHE=/tmp/roroadmap-go-cache go vet ./...
git diff --check
```

- [ ] **Step 3: Commit and push**

```bash
git add docs/social-media-analysis.md docs/superpowers/verification/social-analysis-observability.md
git commit -m "docs: document social analysis trace threading"
git push origin feat/social-media-analysis-metadata-images
```
