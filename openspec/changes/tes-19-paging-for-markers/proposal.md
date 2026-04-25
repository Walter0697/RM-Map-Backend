## Why
The integration marker list currently emphasizes cursor-based iteration. External consumers also need a page-number-based option for workflows that are easier to drive with explicit pages.

## What Changes
- Add a new integration endpoint for marker listing with `page` and `per_page` query parameters.
- Reuse the existing marker filters, sorting rules, and response decoration so both list endpoints stay aligned.
- Return page metadata including `page`, `per_page`, and `total_pages`.

## Impact
- Adds one read-only integration endpoint under the existing markers API surface.
- Keeps the existing cursor-based `/integration/markers` endpoint unchanged for backward compatibility.
