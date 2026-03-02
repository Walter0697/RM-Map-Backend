# Admin Cleanup API Contract

These endpoints are JWT-protected and require an authenticated admin user.
All unauthorized requests return:
- `401 permission denied` for missing/invalid authentication
- `403 permission denied` for authenticated non-admin users

Base path: `/admin/cleanup`

## Search/List Markers
`GET /admin/cleanup/markers`

Supported query controls:
- `limit`, `offset`, `sort_by`, `order`
- `type`, `status`, `country`, `country_code`, `label`, `search`, `relation_id`

Response:
```json
{
  "items": [
    {
      "id": 101,
      "label": "Test Marker",
      "type": "food",
      "status": "",
      "relation_id": 3,
      "country": "Japan",
      "country_code": "JP",
      "to_time": "2026-03-01T02:00:00Z",
      "updated_at": "2026-03-01T10:12:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0,
  "sort_by": "updated_at",
  "order": "desc"
}
```

## Search/List Schedules
`GET /admin/cleanup/schedules`

Supported query controls:
- `limit`, `offset`, `sort_by`, `order`
- `status`, `marker_id`, `label`, `search`, `from`, `to`, `time`, `relation_id`

Response:
```json
{
  "items": [
    {
      "id": 208,
      "label": "Cleanup Candidate",
      "description": "temporary schedule",
      "status": "",
      "relation_id": 3,
      "marker_id": 101,
      "marker_label": "Test Marker",
      "selected_date": "2026-03-01T12:00:00Z",
      "updated_at": "2026-03-01T10:12:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0,
  "sort_by": "selected_date",
  "order": "desc"
}
```

## Permanent Delete Marker
`DELETE /admin/cleanup/markers/{id}`

Request body:
```json
{
  "confirm_id": 101,
  "confirm_label": "Test Marker",
  "reason": "remove test data"
}
```

Response:
```json
{
  "status": "deleted",
  "entity_type": "marker",
  "id": 101
}
```

## Permanent Delete Schedule
`DELETE /admin/cleanup/schedules/{id}`

Request body:
```json
{
  "confirm_id": 208,
  "confirm_label": "Cleanup Candidate",
  "reason": "remove test data"
}
```

Response:
```json
{
  "status": "deleted",
  "entity_type": "schedule",
  "id": 208
}
```

## Schedule Cleanup Job
`POST /admin/cleanup/jobs`

Request body:
```json
{
  "entity_type": "marker",
  "target_id": 101,
  "confirm_id": 101,
  "confirm_label": "Test Marker",
  "execute_at": "2026-03-10T15:04:05Z",
  "reason": "nightly test cleanup"
}
```

Response:
```json
{
  "id": 1,
  "entity_type": "marker",
  "target_id": 101,
  "confirm_label": "Test Marker",
  "reason": "nightly test cleanup",
  "execute_at": "2026-03-10T15:04:05Z",
  "status": "pending",
  "created_at": "2026-03-01T15:00:00Z"
}
```
