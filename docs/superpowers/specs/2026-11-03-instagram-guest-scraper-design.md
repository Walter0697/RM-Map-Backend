# Instagram Guest Scraper Design

## Goal

Make social post scraping match the existing n8n workflow: Instagram uses the public guest GraphQL flow, Threads uses the configured Apify actor, and other URLs use the generic HTML scraper.

## Design

Add an Instagram-specific source adapter implementing the existing `SourceAdapter` interface. It will extract the Instagram shortcode, establish a guest session with a homepage request, read the CSRF cookie, submit the internal GraphQL query, and normalize single media, reels, and carousels into `NormalizedSocialPost`.

The adapter will use an injectable HTTP client and configurable endpoint/doc ID so tests can use an `httptest` server and future Instagram changes do not require changing the adapter structure. Media URLs will be returned with the session metadata needed by the worker's media download path; the adapter itself will not add a separate persistence mechanism.

Source routing will become:

```text
instagram.com → Instagram Guest adapter
threads.net/com → Apify adapter
other hosts → generic HTML adapter
```

The Threads actor remains configured through `APIFY_ACTOR_ID` for the current release. Instagram will not require an Apify actor or Instagram credential.

## Error handling

The adapter will return explicit errors for invalid Instagram URLs, missing cookies, missing CSRF tokens, blocked/empty GraphQL responses, malformed payloads, and oversized responses. It will not attempt proxy rotation or bypass Instagram blocks.

## Testing

Tests will cover shortcode extraction, guest handshake headers/cookie handling, GraphQL normalization for image/video/carousel posts, malformed responses, and source routing. Existing Apify and generic scraper tests must continue to pass.
