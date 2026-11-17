# Telegram Marker Map Notification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send a TomTom static map preview with the existing Telegram notification when a marker is successfully created.

**Architecture:** Keep marker creation and TomTom preview generation in the backend. Extend the Telegram notifier with `sendPhoto`; generate the existing user-specific static preview before sending the success notification, then send the marker details/link as text. If preview generation fails, preserve the existing text notification and log the preview failure.

**Tech Stack:** Go, net/http, Telegram Bot API, existing TomTom static-map preview service, httptest.

## Global Constraints

- Reuse the existing TomTom static preview implementation and user preview-pin/marker-type composition.
- Preserve text-only notifications for conflicts, failures, unlinked users, and preview-generation failures.
- Do not modify the n8n workflow.
- Do not expose Telegram or TomTom secrets in errors or logs.

---

### Task 1: Add failing Telegram photo notification tests

**Files:**
- Modify: `service/telegram_notification_test.go`

**Interfaces:**
- Consumes: `newTelegramNotifier`, `NotifyMarkerCreated`, and the existing static-preview function seam.
- Produces: Regression coverage requiring one `sendPhoto` request followed by the existing `sendMessage` request, plus fallback behavior when preview generation fails.

- [x] **Step 1: Write the failing success-path test**

  Extend the Telegram test server to accept `/botbot-token/sendPhoto`, decode `chat_id`, `photo`, and `caption`, and assert that the photo is sent before the text message. Override the preview-generation seam to return deterministic PNG bytes for a marker with coordinates and a marker type.

- [x] **Step 2: Run the focused test to verify it fails**

  Run: `go test ./service -run TestTelegramNotificationSendsMarkerCreatedMapPreview -count=1`

  Expected: FAIL because `NotifyMarkerCreated` currently only calls `sendMessage`.

- [x] **Step 3: Write the failing fallback test**

  Override the preview-generation seam to return an error, call `NotifyMarkerCreated`, and assert that the text notification still succeeds while no photo request is required.

- [x] **Step 4: Run the focused fallback test to verify it fails or exposes the missing fallback behavior**

  Run: `go test ./service -run TestTelegramNotificationFallsBackWhenMarkerMapPreviewFails -count=1`

  Expected: FAIL until the notifier implements preview fallback.

### Task 2: Implement preview generation and Telegram photo delivery

**Files:**
- Modify: `service/telegram_notification.go`
- Modify: `service/telegram_notification_test.go`

**Interfaces:**
- Consumes: `GenerateStaticMapPreviewByUsername`, `StaticPreviewToBase64`, marker coordinates/type, and the resolved Telegram chat ID.
- Produces: `telegramNotifier.NotifyMarkerCreated` that sends a photo when possible and always attempts the existing text summary.

- [x] **Step 1: Add a notifier-level preview seam**

  Define a package-level function variable wrapping `GenerateStaticMapPreviewByUsername`, so tests can provide deterministic preview bytes and restore the original function with `t.Cleanup`.

- [x] **Step 2: Add `sendPhoto` support**

  Add a small `NotifyPhoto`/`sendPhoto` helper that posts JSON to `/bot<token>/sendPhoto` with `chat_id`, base64-independent photo input suitable for Telegram, and a caption. Use the existing HTTP client and safe error messages.

- [x] **Step 3: Update marker-created notification order**

  Resolve the Telegram ID once. Attempt the static preview with the marker username/type/coordinates. If successful, send the PNG to Telegram, then send the current marker details/link message. If preview generation or photo sending fails, log the failure and still send the text message.

- [x] **Step 4: Run focused tests**

  Run: `go test ./service -run 'TestTelegramNotification(SendsMarkerCreatedMapPreview|FallsBackWhenMarkerMapPreviewFails|SendsMarkerCreatedMessage|SkipsUnlinkedUser)$' -count=1`

  Expected: PASS.

### Task 3: Verify the complete backend change

**Files:**
- No additional files.

- [x] **Step 1: Run formatting and all backend tests**

  Run: `gofmt -w service/telegram_notification.go service/telegram_notification_test.go && go test ./...`

  Expected: PASS with no test failures.

- [x] **Step 2: Review the diff and working tree**

  Run: `git diff --check && git status --short && git diff --stat`

  Expected: Only the notifier, its tests, and this plan are changed; no secrets or generated runtime data appear in the diff.

- [x] **Step 3: Commit the implementation**

  Run: `git add service/telegram_notification.go service/telegram_notification_test.go docs/superpowers/plans/2026-11-17-telegram-marker-map-notification.md && git commit -m "feat: send marker map previews to Telegram"`
