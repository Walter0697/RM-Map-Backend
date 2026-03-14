# Testing Entity Classification Runbook

## Scope

This rollout introduces a `testing` boolean on:

- `api_keys`
- `markers`
- `schedules`

Default is `false`, so existing data remains production by default.

## Apply Migration

No manual SQL step is required for this change.

The backend startup path already runs `dbmodel.AutoMigration()`, which applies
schema changes for `api_keys`, `markers`, and `schedules` automatically.

Rollout step:

1. Deploy backend code with this change.
2. Restart backend so startup AutoMigrate executes.

## Verification Checklist

1. Confirm all three tables contain `testing` with default `false` after startup migration.
2. Create one testing marker/schedule/API key through admin paths and verify `testing=true` persists.
3. Verify non-admin list/read paths do not return testing markers/schedules.
4. Verify admin cleanup route `POST /admin/cleanup/testing/clear` deletes testing markers and schedules only.
5. Verify testing API keys remain after clear action.

## Rollback

If rollback is required:

1. Disable calls to testing clear action at UI/API gateway level.
2. Export rows where `testing=true` for forensic traceability.
3. Drop `testing` columns only after export (manual SQL):
   - `ALTER TABLE api_keys DROP COLUMN testing;`
   - `ALTER TABLE markers DROP COLUMN testing;`
   - `ALTER TABLE schedules DROP COLUMN testing;`

Rollback removes environment classification history, so prefer forward-fix when possible.
