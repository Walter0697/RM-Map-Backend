# Social Media Analysis Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve tags and existing marker metadata fields when the backend social-media analysis flow creates a marker.

**Architecture:** Extend the structured agent result with tags, price, booking, duration, and optional event times. Normalize tags into the existing description-based hashtag convention, and pass all supported values through `CreateMarker`, which already persists and validates these fields.

**Tech Stack:** Go, existing GraphQL marker input, GORM, wcodex structured output.

## Global Constraints

- Existing marker columns and `CreateMarker` behavior are reused; no database migration is required.
- Optional metadata remains empty/null when the source does not provide reliable information.
- Tags are normalized as `#tag` tokens in the description because the existing hashtag API derives tags from marker descriptions.
- Existing analysis results without the new fields remain decodable.

---

### Task 1: Add metadata fields and marker mapping

**Files:**
- Modify: `service/social_media_analysis_types.go`
- Modify: `service/social_media_analysis_worker.go`
- Test: `service/social_media_analysis_worker_test.go`

- [ ] Write failing tests for decoding metadata and mapping it to `model.NewMarker`.
- [ ] Run the focused tests and confirm they fail because the fields are absent.
- [ ] Add the fields, normalize tags, append hashtag tokens to the description, and map price/booking/duration/times into the marker input.
- [ ] Run focused tests and confirm they pass.

### Task 2: Extend the agent prompt and strict output schema

**Files:**
- Modify: `service/social_media_analysis_worker.go`
- Test: `service/social_media_analysis_worker_test.go`

- [ ] Add a failing test requiring the prompt and schema to describe all metadata fields.
- [ ] Add schema definitions and prompt guidance for conservative extraction and empty/null fallbacks.
- [ ] Run the focused tests, full Go tests, and `go vet`.

### Task 3: Rebuild and verify the local service

**Files:**
- No additional production files.

- [ ] Rebuild and restart the local backend with persistent image mounts.
- [ ] Verify backend and frontend health endpoints.
- [ ] Confirm the existing n8n workflows remain active and compatible with the expanded marker payload.
