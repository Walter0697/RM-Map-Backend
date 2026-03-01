#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="${1:-postgres-postgres-1}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-mapmarker}"
SQL_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/migrations/reset_train_station_line_model.sql"

echo "Applying migration: ${SQL_FILE}"
echo "Container: ${CONTAINER_NAME}, DB: ${DB_NAME}, User: ${DB_USER}"

docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" < "${SQL_FILE}"

echo "Migration completed successfully."
