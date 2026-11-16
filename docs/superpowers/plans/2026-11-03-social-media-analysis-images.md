# Social Media Analysis Images Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve an analyzed source image URL and attach the downloaded image to markers created by both the backend social-media pipeline and the existing n8n workflow.

**Architecture:** Extend the structured agent result with an optional `image_url`. The backend validates and forwards that URL through the existing `CreateMarker` service, which already downloads marker images. The n8n workflow will use the same optional field when calling the marker-creation API; image download failure remains non-fatal to marker creation.

**Tech Stack:** Go, GORM models, existing marker service, Go tests, n8n REST API.

## Global Constraints

- `image_url` is optional; a missing or unusable image must not prevent marker creation.
- Do not store secrets in source files, test output, or committed configuration.
- Reuse `CreateMarker` and its existing image download behavior rather than writing marker rows directly.
- Preserve compatibility with existing agent results and n8n payloads that omit image data.

---

### Task 1: Add backend image result and marker propagation

**Files:**
- Modify: `service/social_media_analysis_types.go`
- Modify: `service/social_media_analysis_worker.go`
- Test: `service/social_media_analysis_worker_test.go`

**Interfaces:**
- Consumes: agent JSON containing optional `image_url`.
- Produces: `AgentAnalysisResult.ImageURL` and a `model.NewMarker.ImageLink` pointer passed to `CreateMarker`.

- [ ] **Step 1: Write the failing test**

  Add a test that unmarshals an agent result containing `image_url` and verifies marker input construction preserves the URL. Add a second test proving an omitted `image_url` remains nil and does not change the existing result contract.

- [ ] **Step 2: Run the focused test to verify it fails**

  Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test.*Image' -count=1`

  Expected: FAIL because the result type and marker construction do not yet expose or forward the image URL.

- [ ] **Step 3: Implement the minimal code**

  Add `ImageURL string` with JSON key `image_url` to `AgentAnalysisResult`. In `createSocialMediaAnalysisMarker`, trim the value, take its address only when non-empty, and set `ImageLink` on `model.NewMarker`. Keep the existing `CreateMarker` call and let its image download error remain the marker-creation error path for now; the worker will classify it as a failed job rather than silently claiming an image was saved.

- [ ] **Step 4: Run the focused test to verify it passes**

  Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test.*Image' -count=1`

  Expected: PASS.

- [ ] **Step 5: Commit**

  Run: `git add service/social_media_analysis_types.go service/social_media_analysis_worker.go service/social_media_analysis_worker_test.go && git commit -m "feat: attach analyzed images to markers"`

### Task 2: Update backend agent prompt and schema

**Files:**
- Modify: `service/social_media_analysis_worker.go`
- Test: `service/social_media_analysis_worker_test.go`

**Interfaces:**
- Consumes: normalized page content and extracted source metadata.
- Produces: agent instructions requiring an optional direct image URL and JSON schema property `image_url`.

- [ ] **Step 1: Write the failing test**

  Add a prompt/schema test asserting the generated prompt contains `image_url`, describes it as optional, and includes it in the structured output schema.

- [ ] **Step 2: Run the focused test to verify it fails**

  Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test.*Prompt|Test.*Schema' -count=1`

  Expected: FAIL because the current prompt and schema do not mention `image_url`.

- [ ] **Step 3: Implement the minimal code**

  Extend the agent schema with nullable/optional `image_url` and update the prompt to prefer the page’s canonical `og:image` or equivalent direct image URL. The prompt must tell the agent not to return page URLs, tracking URLs, or guessed image paths.

- [ ] **Step 4: Run the focused test to verify it passes**

  Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test.*Prompt|Test.*Schema' -count=1`

  Expected: PASS.

- [ ] **Step 5: Run the backend regression suite**

  Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./... && GOCACHE=/tmp/roroadmap-go-cache go vet ./...`

  Expected: PASS.

### Task 3: Inspect and update the n8n workflow

**Files:**
- External: the existing n8n Social Media Post Analysis workflow identified through `N8N_BASE_URL` and `N8N_API_TOKEN`.
- Local documentation: `docs/social-media-analysis.md` if the workflow contract is documented there.

**Interfaces:**
- Consumes: n8n analysis output with optional `image_url`.
- Produces: marker creation request with optional `image_link` mapped from `image_url`.

- [ ] **Step 1: Discover the workflow through the n8n API**

  Query the authenticated workflow list and locate the workflow named `Roroadmap Social Media Post Analysis`; inspect only node names, types, and parameter structure. Do not print tokens or full secret-bearing node configuration.

- [ ] **Step 2: Write the workflow contract check**

  Capture the existing analysis output field and marker request mapping. The desired mapping is:

  ```text
  analysis.image_url -> marker.image_link
  ```

  The mapping must be conditional so an absent image URL leaves `image_link` absent/null.

- [ ] **Step 3: Update the workflow**

  Modify the relevant structured-output/parser and marker request nodes. Preserve all existing node IDs, credentials, retry settings, and non-image fields. Do not embed any new secret in node parameters.

- [ ] **Step 4: Validate the workflow definition**

  Re-fetch the workflow and verify the image field is present in the analysis output and marker request mapping without exposing credential values.

- [ ] **Step 5: Document the compatibility contract**

  Add the optional `image_url` → `image_link` mapping to `docs/social-media-analysis.md` if it is not already documented.

### Task 4: End-to-end verification

**Files:**
- Test/verification only; no additional production files expected.

- [ ] **Step 1: Rebuild and restart the local backend image**

  Rebuild `roroadmap-backend:test`, preserving the existing config, uploads, assets, host networking, and wcodex host mapping.

- [ ] **Step 2: Submit a fresh source URL**

  Use the existing authenticated/API-key test path after the user provides the next URL. Confirm the completed job includes a marker ID.

- [ ] **Step 3: Verify image persistence**

  Query the created marker and confirm `image_link` is populated. Request `/image/markers/<saved-file>` and verify HTTP 200 with an image content type.

- [ ] **Step 4: Verify the no-image fallback**

  Confirm a source with no valid direct image still creates a marker with an empty image link and a completed analysis status.
