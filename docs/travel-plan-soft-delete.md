# Travel Plan Soft Delete API

This document describes soft-delete behavior for travel plans and travel plan items.

## Integration API (API key)

Required scope for delete endpoints: `travel-plans:write`.

### Delete a travel plan

- Method: `DELETE`
- Path: `/integration/travel-plans/{id}`
- Behavior:
  - Soft-deletes the parent travel plan.
  - Soft-deletes all child daily-plan items for the plan.
  - Subsequent default read/list endpoints no longer return the deleted plan.

Response example:

```json
{
  "deleted": true,
  "entity": "travel_plan",
  "id": 97
}
```

### Delete a travel plan item

- Method: `DELETE`
- Path: `/integration/travel-plans/{id}/daily-plans/{daily_id}`
- Behavior:
  - Soft-deletes one item under the target plan.
  - Subsequent default detail reads no longer include the deleted item.

Response example:

```json
{
  "deleted": true,
  "entity": "travel_plan_daily",
  "id": 301,
  "travel_plan_id": 97
}
```

## User API (JWT)

### Delete a travel plan

- Method: `DELETE`
- Path: `/travel-plans/{id}`
- Ownership check:
  - Requesting user must own the plan.
- Behavior:
  - Soft-deletes plan and all child items.

### Delete a travel plan item

- Method: `DELETE`
- Path: `/travel-plans/{id}/daily-plans/{daily_id}`
- Ownership check:
  - Requesting user must own the parent plan.
- Behavior:
  - Soft-deletes the target item.

## Default read filtering

Travel plan list/detail endpoints use default GORM filtering, so rows with `deleted_at` set are excluded from normal reads.
