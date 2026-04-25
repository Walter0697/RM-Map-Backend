# TES-0019 - Paging for markers

## Summary

Add a page-based integration endpoint for marker listing alongside the existing cursor-based endpoint so clients can choose either pagination model.

## Problem

The integration API currently exposes marker listing through a cursor-oriented contract. Some consumers need explicit page navigation with page metadata instead of cursor traversal.

## Proposal

- Add `GET /integration/markers/paged`.
- Support the existing marker filters and sorting options.
- Accept `page` and `page_size` query parameters.
- Return `items`, `total`, `page`, `page_size`, `total_pages`, `has_next`, and `has_prev`.
- Keep the existing `/integration/markers` endpoint unchanged for backward compatibility.

## Impact

- No breaking change for current cursor-based clients.
- New integrations can adopt simpler page-number navigation when that matches their UI or data-fetching model better.
