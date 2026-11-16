# Instagram Guest Scraper Design

## Goal

Make social post scraping reliable for public social URLs: Instagram and Threads use source-specific Apify actors, and other URLs use the generic HTML scraper.

## Design

Use the existing Apify source adapter with source-specific actor credentials and input formats. Instagram sends `directUrls`, `resultsType: posts`, and `resultsLimit: 1`; Threads sends `post_urls`. Both results normalize into `NormalizedSocialPost`.

The adapter will use an injectable HTTP client and configurable endpoint/doc ID so tests can use an `httptest` server and future Instagram changes do not require changing the adapter structure. Media URLs will be returned with the session metadata needed by the worker's media download path; the adapter itself will not add a separate persistence mechanism.

Source routing will become:

```text
instagram.com → Instagram Apify adapter
threads.net/com → Apify adapter
other hosts → generic HTML adapter
```

Instagram uses `APIFY_INSTAGRAM_ACTOR_ID`; Threads uses `APIFY_THREADS_ACTOR_ID`. `APIFY_ACTOR_ID` remains a backward-compatible fallback. Neither source requires Instagram credentials.

## Error handling

The adapter will return explicit errors for missing actor configuration, failed actor runs, empty datasets, malformed payloads, and oversized responses. It will not attempt proxy rotation or bypass Instagram blocks itself; Apify owns that concern.

## Testing

Tests will cover shortcode extraction, guest handshake headers/cookie handling, GraphQL normalization for image/video/carousel posts, malformed responses, and source routing. Existing Apify and generic scraper tests must continue to pass.
