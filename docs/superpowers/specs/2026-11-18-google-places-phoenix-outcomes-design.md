# Google Places Phoenix Outcomes

## Goal

Keep every Google Places verification outcome attached to the same social-analysis `analysis.run` trace so quota, billing, API-key, transport, fallback, and success behavior can be diagnosed together.

## Approved behavior

- Google Places emits a child span under the caller's existing analysis context.
- The span records a safe outcome category and HTTP/API status metadata, without logging the API key or raw location query.
- Quota, rate-limit, billing, and key errors are classified separately from transport, decode, and generic HTTP failures.
- Google remains optional: any Google failure preserves the existing agent-only analysis result.
- The fallback decision and final analysis outcome remain in the same `analysis.run` trace.

## Testing

Tests will cover structured Google error classification and trace attributes for success and failure paths. The existing full Go test suite remains the final verification gate.
