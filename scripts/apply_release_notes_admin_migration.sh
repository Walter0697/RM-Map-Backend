#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="${1:-postgres-postgres-1}"
DATABASE_NAME="${2:-mapmarker}"
USER_NAME="${3:-postgres}"
SQL_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/migrations/add_release_notes_admin_fields.sql"

echo "Applying migration: ${SQL_FILE}"
docker exec -i "${CONTAINER_NAME}" psql -U "${USER_NAME}" -d "${DATABASE_NAME}" < "${SQL_FILE}"
echo "Migration complete"
