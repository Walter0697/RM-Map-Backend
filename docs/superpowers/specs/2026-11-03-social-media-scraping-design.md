# Social Media Scraping Design

## Goal

Complete the source-acquisition phase of social media analysis before adding
Google Maps or web-search verification. A submitted URL must be fetched and
normalized synchronously within the analysis job before the wcodex/OpenAI
agent is invoked.

## Source routing

The backend selects a scraper from the normalized URL host:

- Instagram URLs use an Apify-backed adapter.
- Threads URLs use an Apify-backed adapter.
- All other public HTTP(S) URLs use the existing bounded generic fetcher.

The API remains unchanged: phase one still accepts only `{"url":"..."}` and
returns an asynchronous analysis job. “Synchronous scraping” means source
acquisition completes as one stage of that job; it does not make the HTTP
request wait for the entire analysis pipeline.

## Normalized scraper output

Every adapter returns the same internal source model containing:

- source and final canonical URLs
- title and description text
- bounded page/body text
- extracted hashtags
- canonical image URLs
- video URLs or provider media references
- source metadata needed by the downstream agent

The generic adapter extracts standard HTML metadata such as `title`,
description, `og:image`, `og:video`, and visible hashtag tokens. Apify output
is mapped into the same model. Provider-specific payloads are not passed
through to the API response or persisted unbounded.

## Analysis flow

1. Validate the submitted public URL.
2. Select the source adapter.
3. Fetch and normalize the source content.
4. Persist the bounded normalized source payload and stage.
5. Pass text, hashtags, image references, and video references to the agent.
6. Let the agent produce the marker analysis and normalized hashtag output.
7. Create the marker using the existing marker path.

There is no Google Maps lookup in this phase. Location confirmation and web
search will be added as a later verification stage for uncertain candidates.

## Secrets and Infisical

The backend uses Infisical Universal Auth. TOML contains only non-secret
configuration and secret references:

- `enabled`
- `siteurl`
- `projectid`
- `environment`
- `secretpath`
- `clientid`
- the environment-variable name containing the Universal Auth client secret

Apify and Gemini tokens are retrieved through the configured Infisical
runtime injection path. Secret values must not be committed to `config.toml`,
workflow definitions, prompts, job metadata, or logs.

The first implementation will add the Infisical configuration contract and
Apify source boundary. Gemini will be represented as an optional provider
configuration boundary but will not be invoked unless a later media-analysis
decision requires it.

## Error handling

- Apify failure is reported as a source-fetch failure for Instagram/Threads;
  the job must not silently fall back to generic scraping for those hosts.
- Generic fetch failures remain bounded and classified as source-fetch
  failures.
- Missing provider credentials fail clearly at the source/provider boundary
  without exposing secret values.
- Oversized or malformed provider responses are rejected before persistence.

## Testing

Unit tests will cover host routing, generic metadata extraction, hashtag
normalization, Apify response mapping, Infisical configuration validation,
provider credential absence, bounded payloads, and the complete job ordering:
scrape/normalize first, agent submission second.

