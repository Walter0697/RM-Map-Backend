# Release Note Migration Dry-Run Report (2026-03-07)

## Scope
OpenSpec task `manage-release-notes-in-admin` task 4.3:
- run migration dry-run
- verify ordering/version consistency of migrated release notes

## Environment
- Workspace: `.agent-workspaces/feat-rrm-0010/RM-Map-Backend`
- Database container: `postgres-postgres-1`
- Database: `mapmarker`

## Commands Run
- `./scripts/apply_release_notes_admin_migration.sh postgres-postgres-1 mapmarker postgres`
- `go run scripts/backfill_release_notes.go`
- SQL consistency checks via `docker exec ... psql`

## Results
1. Migration script executed idempotently:
- Existing columns/indexes were skipped with NOTICE
- `UPDATE 0` for migration publish-state backfill step

2. Backfill execution result:
- `backfill complete, updated=16`

3. Version-format consistency:
- `total_notes=18`
- `semver_like_notes=18`
- `non_semver_notes=0`

4. Duplicate-version check:
- no duplicate versions found

5. Ordering/version sample verification:
- release notes are queryable in semantic version descending order (`2.9.5` down to `2.0.0` in sampled output)
- highest visible version: `2.9.5`
- all sampled rows showed expected release-note metadata fields populated (`publish_state`, `notes_format`, timestamps)

## Conclusion
Dry-run and verification passed. Migration/backfill behavior was successful, semver/version consistency was validated, and ordering by version is consistent with expected output.
