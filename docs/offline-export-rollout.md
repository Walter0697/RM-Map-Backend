# Offline Export Rollout

The offline export feature is gated separately in backend and frontend so rollout can happen in stages.

## Backend

- `OFFLINE_EXPORT_ENABLE=true|false`
  - `false` disables create/status/artifact endpoints.
- `OFFLINE_EXPORT_FORMATS=text,image`
  - Controls which formats the backend accepts.
  - If unset, backend defaults to `text,image`.

## Frontend

- `REACT_APP_OFFLINE_EXPORT_ENABLED=true|false`
  - `false` hides the export entrypoint on the schedule screen.
- `REACT_APP_OFFLINE_EXPORT_FORMATS=text,image`
  - Controls which format checkboxes are shown in the export dialog.
  - If unset, frontend defaults to `text,image`.

## Default Enablement

Without extra env configuration, the app exposes all export formats:

1. `text`
2. `image`

## Rollback

- Immediate kill switch: set `OFFLINE_EXPORT_ENABLE=false` and `REACT_APP_OFFLINE_EXPORT_ENABLED=false`.
- Format-specific rollback: remove the affected format from both `OFFLINE_EXPORT_FORMATS` and `REACT_APP_OFFLINE_EXPORT_FORMATS`.
- Safe baseline: set both format lists to `text`.
