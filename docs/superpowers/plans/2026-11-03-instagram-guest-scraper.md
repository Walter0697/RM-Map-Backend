# Instagram Guest Scraper Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route Instagram URLs through an n8n-compatible guest/GraphQL scraper while retaining Apify for Threads and generic scraping elsewhere.

**Architecture:** Add a focused `instagramGuestClient` implementing `SourceAdapter`, with an injectable HTTP client and endpoint/doc ID. Update source selection so only Threads constructs an Apify adapter; Instagram constructs the guest adapter.

**Tech Stack:** Go, `net/http`, `encoding/json`, `net/http/httptest`, existing `SourceAdapter` and `NormalizedSocialPost` types.

## Global Constraints

- Do not require Instagram credentials or an Instagram actor ID.
- Keep public URL validation and existing response-size limits.
- Do not add proxy rotation or anti-blocking bypass behavior.
- Preserve Threads Apify configuration through `APIFY_TOKEN` and `APIFY_ACTOR_ID`.

---

### Task 1: Add failing Instagram Guest adapter tests

**Files:**
- Create: `service/social_media_analysis_instagram_test.go`
- Modify: `service/social_media_analysis_source_test.go`

**Interfaces:**
- Tests will define the expected `NewInstagramGuestClient(baseURL, docID string, client *http.Client) SourceAdapter` behavior and routing result.

- [x] **Step 1: Write tests for shortcode extraction and image normalization**

  Use an `httptest.Server` to assert the homepage request returns `csrftoken`, then assert the GraphQL request receives the expected shortcode, `doc_id`, CSRF header, and cookie. Return a fixture containing `XDTGraphSidecar` with image children and assert caption, image URLs, and hashtags are normalized.

- [x] **Step 2: Add video and invalid-response cases**

  Return a single `is_video` media object and assert video URL plus thumbnail. Add cases for a URL without an Instagram post shortcode and a GraphQL response without `xdt_shortcode_media`, expecting errors.

- [x] **Step 3: Run the focused tests and verify they fail**

  Run `go test ./service -run 'InstagramGuest|SelectConfiguredSourceAdapter'`. Expected result: compile/test failure because the client and routing implementation do not exist yet.

### Task 2: Implement the Instagram Guest adapter

**Files:**
- Create: `service/social_media_analysis_instagram.go`

**Interfaces:**
- Produces `instagramGuestClient` implementing `SourceAdapter`.

- [x] **Step 1: Implement shortcode extraction**

  Accept `/p/{shortcode}`, `/reel/{shortcode}`, `/reels/{shortcode}`, and `/tv/{shortcode}` paths. Reject empty or unsupported Instagram paths with `ErrInvalidSourceURL`-wrapped errors.

- [x] **Step 2: Implement the guest handshake**

  Request the configured base URL homepage with the browser User-Agent, require a successful response, and extract the `csrftoken` cookie. Use a response-cookie jar or equivalent request-scoped cookie handling so the GraphQL request sends the session cookie.

- [x] **Step 3: Implement the GraphQL request and normalization**

  POST form-encoded `variables` and `doc_id` to `/graphql/query`, set `X-CSRFToken`, `Content-Type`, and User-Agent, then map single media and sidecar children to `NormalizedSocialPost.ImageURLs` and `.VideoURLs`. Map caption, owner metadata, final URL, and deduplicated hashtags.

- [x] **Step 4: Run focused tests and verify they pass**

  Run `go test ./service -run 'InstagramGuest'`. Expected result: PASS.

### Task 3: Route sources correctly

**Files:**
- Modify: `service/social_media_analysis_apify.go`
- Modify: `service/social_media_analysis_source_test.go`

**Interfaces:**
- `SelectConfiguredSourceAdapter` returns the Instagram Guest adapter for Instagram hosts, Apify for Threads hosts, and generic scraping for other hosts.

- [x] **Step 1: Update routing tests**

  Assert Instagram returns `instagram-guest`, Threads returns `apify`, and generic URLs return `generic`.

- [x] **Step 2: Update source selection implementation**

  Construct `NewInstagramGuestClient` for Instagram using the configured guest endpoint/doc ID defaults. Leave the existing Apify construction only in the Threads branch.

- [x] **Step 3: Run all service tests**

  Run `go test ./service`. Expected result: PASS.

### Task 4: Verify the complete backend

**Files:**
- No production file changes expected.

- [x] **Step 1: Run formatting and static checks**

  Run `gofmt -w service/social_media_analysis_instagram.go service/social_media_analysis_instagram_test.go service/social_media_analysis_apify.go service/social_media_analysis_source_test.go`, then `go test ./...` and `go vet ./...`.

- [x] **Step 2: Review the diff**

  Run `git diff --check` and `git status --short`; confirm no secrets or unrelated files are included.
