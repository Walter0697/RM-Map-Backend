#!/usr/bin/env bash
set -euo pipefail

# Smoke check for integration marker create/list flows with provider-linked website IDs.
# Required environment variables:
# - API_BASE (e.g. http://localhost:1416/integration)
# - API_KEY

if [[ -z "${API_BASE:-}" || -z "${API_KEY:-}" ]]; then
  echo "API_BASE and API_KEY are required"
  exit 1
fi

echo "== create marker with OpenRice provider =="
curl -sS -X POST "${API_BASE}/markers" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "label":"OpenRice smoke marker",
    "latitude":22.3034,
    "longitude":114.1718,
    "address":"HK",
    "type":"food",
    "website_provider":"openrice",
    "website_provider_id":"en/hongkong/r-demo"
  }' | sed -e 's/},{/},\n{/g'

echo
echo "== create marker with Yelp provider =="
curl -sS -X POST "${API_BASE}/markers" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "label":"Yelp smoke marker",
    "latitude":43.7615,
    "longitude":-79.4111,
    "address":"North York",
    "type":"food",
    "website_provider":"yelp",
    "website_provider_id":"north-york-cafe"
  }' | sed -e 's/},{/},\n{/g'

echo
echo "== create marker with Tabelog provider =="
curl -sS -X POST "${API_BASE}/markers" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "label":"Tabelog smoke marker",
    "latitude":35.6895,
    "longitude":139.6917,
    "address":"Tokyo",
    "type":"food",
    "website_provider":"tabelog",
    "website_provider_id":"tokyo/A1304/A130401/13000001"
  }' | sed -e 's/},{/},\n{/g'

echo
echo "== list markers (verify website_integration payload) =="
curl -sS "${API_BASE}/markers?limit=5&sort_by=updated_at&order=desc" \
  -H "X-API-Key: ${API_KEY}" | sed -e 's/},{/},\n{/g'
