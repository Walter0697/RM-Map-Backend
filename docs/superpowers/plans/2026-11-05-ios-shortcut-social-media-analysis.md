# iOS Shortcut Social Media Analysis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add a native Roroadmap Settings flow that installs a generic iOS Shortcut and submits shared URLs to the existing social-media analysis endpoint.

**Architecture:** Add a backend-owned, one-time setup-token exchange that creates a relation-scoped API key with only `social-media:analysis`. Serve the generic Shortcut artifact from an authenticated backend download endpoint, and update the React Settings page to download it, launch setup through `shortcuts://`, and show only a fire-and-forget confirmation after submission.

**Tech Stack:** Go, Chi, GORM, existing JWT/API-key authentication, React, Material UI, Jest/React Testing Library, existing iOS Shortcut artifact from TubeMin.

## Global Constraints

- The Shortcut submission is asynchronous; its success message is exactly `Feature requested — analysing.`.
- The Shortcut must submit through `POST /integration/social-media/analyses` using `X-API-Key`.
- Setup tokens expire after 10 minutes and are single-use.
- Persist only setup-token hashes; never persist or log the generated API key plaintext.
- Generated API keys receive only `social-media:analysis` and the authenticated user's relation.
- Do not change the existing scraping, agent, Google Places, marker, or Telegram pipelines.
- Do not put API keys in URLs, HTML, logs, analytics attributes, or Phoenix span attributes.
- Do not modify the signed Shortcut artifact by string replacement or embed a secret in it.
- Preserve the existing `ios_shortcut_install_url` admin setting until the native flow is verified; the user-facing Settings CTA will use the native flow.

---

## Repository and file map

Backend repository: `/home/git/RoroadMap/RM-Map-Backend`

- `database/dbmodel/social_media_shortcut_setup_token.go`: GORM model for hashed, expiring setup tokens.
- `service/social_media_shortcut.go`: token creation/consumption and Shortcut API-key provisioning.
- `service/social_media_shortcut_http.go`: authenticated setup/download handlers and JSON response types.
- `server.go`: register Settings and public setup routes.
- `database/dbmodel/base.go`: include the setup-token model in existing auto-migration if required by the project migration pattern.
- `assets/roroadmap-ios-shortcut.shortcut`: checked-in copy of the generic TubeMin artifact, copied unchanged and verified byte-for-byte. The installed shortcut name remains `Send To Tubemin` until a separately branded artifact is available.
- `service/social_media_shortcut_test.go`: token and key-provisioning tests.
- `service/social_media_shortcut_http_test.go`: route/auth/response tests.

Frontend repository: `/home/git/RoroadMap/RM-Map-Frontend`

- `src/pages/SettingPage.js`: replace the legacy external shortcut launch with native setup/download/launch behavior.
- `src/components/list/settings/SettingList.js`: keep the existing Settings CTA wiring, changing only its label if needed.
- `src/pages/SettingPage.test.js`: test setup request, artifact download, URL-scheme launch, and user-visible success/error states.
- `public/assets/shortcut-guide/*`: update only if existing Telegram-oriented screenshots are retained; do not make the first implementation depend on new screenshots.

TubeMin reference repository: `/home/git/tubemin`

- Source artifact: `server/assets/tubemin-ios-shortcut.shortcut`.
- Copy the artifact without transformation; use the existing TubeMin setup response contract (`submit_url`, `api_key`).

---

### Task 1: Add persisted setup-token ownership

**Files:**
- Create: `RM-Map-Backend/database/dbmodel/social_media_shortcut_setup_token.go`
- Modify: `RM-Map-Backend/database/dbmodel/base.go` only if the existing AutoMigrate list is explicit
- Test: `RM-Map-Backend/service/social_media_shortcut_test.go`

**Interfaces:**
- Produce `CreateSocialMediaShortcutSetupToken(userID uint, relationID uint) (plaintext string, err error)`.
- Produce `ConsumeSocialMediaShortcutSetupToken(plaintext string) (owner SocialMediaShortcutSetupOwner, found bool, err error)`.
- `SocialMediaShortcutSetupOwner` must include `UserID` and `RelationID`.

- [ ] **Step 1: Write failing token lifecycle tests.**

Test these exact behaviors: a generated token is accepted once; a second consume returns `found == false`; an expired token returns `found == false`; the database row contains only a hash and no plaintext token.

- [ ] **Step 2: Run the focused tests and confirm they fail because the model/functions do not exist.**

Run:

```bash
cd /home/git/RoroadMap/RM-Map-Backend
go test ./service -run 'SocialMediaShortcutSetupToken' -count=1
```

Expected: compilation failure naming the missing model or service functions.

- [ ] **Step 3: Implement the model and token service.**

Use the existing GORM connection and hashing utility. Generate at least 32 random bytes from `crypto/rand`, encode the plaintext URL-safely, hash the plaintext before persistence, set `ExpiresAt` to `time.Now().Add(10 * time.Minute)`, and delete the matching row inside the consume transaction. Store `UserID`, `RelationID`, `TokenHash`, `CreatedAt`, and `ExpiresAt`.

- [ ] **Step 4: Run the focused tests and inspect the schema behavior.**

Run the focused test command again and verify that the tests pass and auto-migration creates the setup-token table in the test database.

- [ ] **Step 5: Commit the token persistence unit.**

```bash
git add database/dbmodel/social_media_shortcut_setup_token.go database/dbmodel/base.go service/social_media_shortcut_test.go
git commit -m "feat: add social shortcut setup tokens"
```

### Task 2: Provision a scoped API key through the setup exchange

**Files:**
- Modify: `RM-Map-Backend/service/social_media_shortcut.go`
- Test: `RM-Map-Backend/service/social_media_shortcut_test.go`

**Interfaces:**
- Produce `ExchangeSocialMediaShortcutSetupToken(token string) (submitURL string, rawAPIKey string, err error)`.
- The function must call the existing API-key creation path with `constant.APIKeyScopeSocialMediaAnalysis`, the setup owner’s `RelationID`, and the setup owner’s `UserID` as `ActorUserID`.

- [ ] **Step 1: Add failing exchange tests.**

Assert that a valid setup token creates an active key owned by the expected relation, with exactly one scope (`social-media:analysis`), and returns a non-empty raw key. Assert that an invalid or replayed token creates no key.

- [ ] **Step 2: Run the focused tests and confirm failure.**

Run:

```bash
go test ./service -run 'SocialMediaShortcutExchange' -count=1
```

- [ ] **Step 3: Implement the exchange using existing API-key primitives.**

Consume the token first. If no owner is returned, return an invalid-token error without creating a key. Call `CreateAPIKey` with the relation and actor from the token, use a clear name such as `Roroadmap iOS Shortcut`, and pass only `constant.APIKeyScopeSocialMediaAnalysis`. Return the configured analysis endpoint separately from the raw key; do not log either secret or raw key.

- [ ] **Step 4: Run the exchange tests and verify scope/ownership.**

Run the focused tests and use database assertions to confirm the stored key contains a hash, relation ID, actor user ID, active status, and only the analysis scope.

- [ ] **Step 5: Commit the provisioning unit.**

```bash
git add service/social_media_shortcut.go service/social_media_shortcut_test.go
git commit -m "feat: provision scoped shortcut api keys"
```

### Task 3: Add authenticated setup and artifact-download HTTP endpoints

**Files:**
- Create: `RM-Map-Backend/service/social_media_shortcut_http.go`
- Create: `RM-Map-Backend/service/social_media_shortcut_http_test.go`
- Modify: `RM-Map-Backend/server.go`
- Create: `RM-Map-Backend/assets/roroadmap-ios-shortcut.shortcut`

**Interfaces:**
- `POST /settings/ios-shortcut/setup` with the existing JWT `Authorization` header returns JSON `{ "setup_url": "...", "download_url": "..." }`.
- `GET /settings/ios-shortcut/download` with the existing JWT `Authorization` header returns the Shortcut bytes as an attachment.
- `POST /api/shortcut/setup/{token}` consumes a token and returns `{ "submit_url": ".../integration/social-media/analyses", "api_key": "..." }`.

- [ ] **Step 1: Copy and verify the generic Shortcut artifact.**

Copy `/home/git/tubemin/server/assets/tubemin-ios-shortcut.shortcut` to `assets/roroadmap-ios-shortcut.shortcut` without modification, then run:

```bash
cmp /home/git/tubemin/server/assets/tubemin-ios-shortcut.shortcut assets/roroadmap-ios-shortcut.shortcut
```

Expected: exit code 0 with no output.

- [ ] **Step 2: Write failing HTTP tests.**

Test: unauthenticated Settings setup is rejected; authenticated setup returns an absolute setup URL; download returns `200`, an attachment content disposition, and non-empty bytes; setup exchange returns `401` for invalid tokens and returns the analysis endpoint for a valid token; replaying the token returns `401`.

- [ ] **Step 3: Run the HTTP tests and confirm they fail.**

Run:

```bash
go test ./service -run 'SocialMediaShortcutHTTP|SocialMediaShortcutSetupHandler' -count=1
```

- [ ] **Step 4: Implement the handlers and routes.**

Use the existing JWT user extraction and relation lookup patterns for Settings. Build the public setup response from the trusted backend request origin, not an arbitrary request body value. Serve the unchanged artifact with `Content-Type: application/octet-stream` and `Content-Disposition: attachment; filename="Send To Tubemin.shortcut"`. Do not include raw keys in logs or error responses.

- [ ] **Step 5: Run the HTTP tests and verify the complete exchange.**

Run the focused tests again. Confirm the returned key authenticates a request to the existing social-analysis handler and that the handler responds with `202 Accepted`.

- [ ] **Step 6: Commit the backend HTTP unit.**

```bash
git add service/social_media_shortcut_http.go service/social_media_shortcut_http_test.go server.go assets/roroadmap-ios-shortcut.shortcut
git commit -m "feat: add native social analysis shortcut endpoints"
```

### Task 4: Replace the frontend Telegram-oriented shortcut handoff

**Files:**
- Modify: `/home/git/RoroadMap/RM-Map-Frontend/src/pages/SettingPage.js`
- Modify: `/home/git/RoroadMap/RM-Map-Frontend/src/components/list/settings/IOSShortcutInstallButton.js`
- Modify: `/home/git/RoroadMap/RM-Map-Frontend/src/components/list/settings/SocialMediaPostShortcutButton.js`
- Test: `/home/git/RoroadMap/RM-Map-Frontend/src/pages/SettingPage.test.js`

**Interfaces:**
- The page uses the existing JWT from Redux for Settings requests.
- Setup request: `POST settings/ios-shortcut/setup` with `Authorization: <jwt>`.
- Download request: `GET settings/ios-shortcut/download` with `Authorization: <jwt>`.

- [ ] **Step 1: Add failing frontend tests.**

Mock `fetch` and assert that clicking the install control calls the setup endpoint with the JWT, downloads the artifact using the returned setup URL, and launches `shortcuts://run-shortcut` with the setup URL URL-encoded. Assert that a failed request renders a retryable error without rendering any token or API key.

- [ ] **Step 2: Run the focused frontend test and confirm failure.**

Run the repository’s existing test command for the new test file, for example:

```bash
cd /home/git/RoroadMap/RM-Map-Frontend
npm test -- --runInBand src/pages/SettingPage.test.js
```

Use the project’s existing package-manager command if it differs from `npm test`.

- [ ] **Step 3: Implement the native setup flow.**

Replace the current external `ios_shortcut_install_url` launch in the social-media shortcut UI with a backend setup request. Fetch the binary with the JWT, create a `File`, use `navigator.share` when available, and otherwise trigger a normal download. Launch the installed shortcut with a URL of the form:

```js
`shortcuts://run-shortcut?name=\${encodeURIComponent('Send To Tubemin')}&input=text&text=\${encodeURIComponent(setupUrl)}`
```

Keep Telegram’s separate Settings control unchanged. Update the help copy so it says that sharing a URL requests analysis and does not mention opening Telegram or tapping Send.

- [ ] **Step 4: Add the fire-and-forget confirmation behavior.**

The Shortcut artifact itself displays `Feature requested — analysing.` after a successful `2xx` submission. The web Settings page should only show installation/setup status and must not claim that a marker has already been created.

- [ ] **Step 5: Run the focused frontend tests and verify no secret leakage.**

Run the focused test command and inspect rendered error text and mocked network calls to confirm setup tokens, API keys, and response bodies are not rendered.

- [ ] **Step 6: Commit the frontend unit.**

```bash
git add src/pages/SettingPage.js src/components/list/settings/IOSShortcutInstallButton.js src/components/list/settings/SocialMediaPostShortcutButton.js src/pages/SettingPage.test.js
git commit -m "feat: install social analysis shortcut from settings"
```

### Task 5: End-to-end verification and documentation

**Files:**
- Modify: `RM-Map-Backend/docs/social-media-analysis.md`
- Modify: `RM-Map-Frontend/README.md` only if the frontend has user setup documentation
- Test: backend and frontend existing suites

- [ ] **Step 1: Document the setup contract.**

Document the Settings setup route, one-time token lifetime, generated key scope, `X-API-Key` submission header, `202 Accepted` behavior, and the exact Shortcut success message. State that Telegram remains supported but is not required for Shortcut submissions.

- [ ] **Step 2: Run backend formatting and tests.**

```bash
cd /home/git/RoroadMap/RM-Map-Backend
gofmt -w database/dbmodel/social_media_shortcut_setup_token.go service/social_media_shortcut.go service/social_media_shortcut_http.go service/social_media_shortcut_test.go service/social_media_shortcut_http_test.go
go test ./...
go vet ./...
git diff --check
```

- [ ] **Step 3: Run frontend tests and build.**

```bash
cd /home/git/RoroadMap/RM-Map-Frontend
npm test -- --runInBand
npm run build
```

- [ ] **Step 4: Perform manual iPhone acceptance.**

Log into Roroadmap, open Settings, install the Shortcut, complete setup, share an Instagram or Threads URL, confirm the Shortcut displays `Feature requested — analysing.`, and verify the backend creates a queued job that later reaches `marker_created`.

- [ ] **Step 5: Commit documentation and final verification notes.**

```bash
git add docs/social-media-analysis.md
git commit -m "docs: document social analysis shortcut setup"
```

## Self-review checklist

- The plan covers setup-token persistence, scoped key creation, backend routes, artifact delivery, frontend installation, fire-and-forget behavior, and testing.
- The existing backend authentication contract is used: `X-API-Key`, not Bearer JWT, for integration submission.
- Telegram is preserved and only the social-media Shortcut handoff is changed.
- No task requires modifying the signed Shortcut binary or embedding a secret.
- MinIO, Jev, polling, and marker feedback remain explicitly deferred.
