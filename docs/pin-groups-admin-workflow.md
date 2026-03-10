# Admin Pin Group Workflow

This document describes how to manage pin groups and how ungrouped pins behave.

## Admin workflow

1. Open `Admin -> Pin` and click `Manage Groups`.
2. Create group names (for example: `Commuting`, `Restaurants`, `Emergency`).
3. Return to `Admin -> Pin` and create or edit pins.
4. In the `Pin Groups` multi-select, choose zero or more groups.
5. Save the pin.

## Ungrouped pins

- Pins with no group assignment are valid.
- User settings always include an explicit `Ungrouped` section.
- Deleting a group does not delete pins; affected pins move to `Ungrouped`.

## Compatibility and rollout notes

- `GET /settings/pins` returns:
  - `pins`: flat list (backward-compatible behavior)
  - `groups`: grouped list (new behavior for grouped UI)
- Existing clients can continue using the flat `pins` list without changes.

## Query/performance behavior

- Settings grouped data is built from a single pin load path (`GetAllPin`) with group preloading.
- A guard test (`TestSettingsListPinsHandlerSingleLoadCall`) verifies the handler performs one pin-load call per request path.
