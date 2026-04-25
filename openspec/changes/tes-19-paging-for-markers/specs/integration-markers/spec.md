## ADDED Requirements

### Requirement: Page-Based Integration Marker Listing
The system MUST provide an additional integration marker listing endpoint that supports page-number-based pagination.

#### Scenario: Request a paged marker list
- **WHEN** a caller sends `GET /integration/markers/paged` with a valid API key that has `markers:read`
- **AND** the request includes valid `page` and optional `per_page` query parameters
- **THEN** the system returns `200 OK`
- **AND** the response includes `items`, `total`, `page`, `per_page`, and `total_pages`
- **AND** the items are filtered to the API key relation and hidden marker types remain excluded

#### Scenario: Reuse existing marker filters
- **WHEN** a caller sends `GET /integration/markers/paged` with the same marker filters and sort options supported by `GET /integration/markers`
- **THEN** the system applies the same filtering and sorting rules before page slicing

#### Scenario: Reject invalid page input
- **WHEN** a caller sends `GET /integration/markers/paged` with an invalid `page` or `per_page`
- **THEN** the system returns `400 Bad Request`
