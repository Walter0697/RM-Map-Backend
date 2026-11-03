# Social Media Scraping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Route Instagram and Threads URLs through Apify, keep a bounded generic scraper for other URLs, and resolve Apify/Gemini secrets from either Infisical Universal Auth or local TOML configuration.

**Architecture:** The analysis worker will select a `SourceAdapter` by validated hostname. Generic HTML fetching and Apify responses will be normalized into the same bounded `NormalizedSocialPost`, including metadata, hashtags, image URLs, and video URLs, before runner submission. A small secret resolver will prefer Infisical, then explicit environment overrides, then TOML fallback values; Gemini will be configured as an optional provider but not called in this phase.

**Tech Stack:** Go, Viper/TOML configuration, `net/http`, existing GORM job worker, Apify REST API, Infisical Universal Auth REST API, Go test.

## Global Constraints

- The phase-one public API remains `POST /integration/social-media/analyses` with `{"url":"..."}`.
- Scraping is a synchronous stage inside the asynchronous job and must finish before runner submission.
- Only public HTTP(S) URLs accepted by the existing URL safety boundary may be fetched.
- Instagram and Threads Apify failures are source failures; they must not silently fall back to generic scraping.
- Provider responses and persisted normalized content are bounded by configured limits.
- Secret values must not appear in logs, prompts, runner metadata, API responses, or shared example configuration.
- Google Maps/search verification is out of scope for this phase.

---

### Task 1: Add dual-source secret configuration

**Files:**
- Modify: `config/setting.go`
- Modify: `config.example.toml`
- Test: `config/setting_test.go`

**Interfaces:**
- Produce `config.InfisicalSetting` with Universal Auth identity fields and local fallback fields.
- Produce environment overrides for Infisical and provider settings without exposing values in diagnostics.

- [ ] **Step 1: Write failing configuration tests**

Add tests for TOML fallback values, environment overrides, Infisical enablement, and invalid Universal Auth settings. Assert that the resolved configuration preserves the configured secret source without logging or serializing secret values.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./config -run 'Test(Infisical|SocialMediaAnalysis)' -count=1`

Expected: FAIL because the Infisical setting and provider fields do not exist.

- [ ] **Step 3: Implement the configuration contract**

Add fields equivalent to:

```go
type InfisicalSetting struct {
    Enable                  bool   `mapstructure:"enable"`
    SiteURL                 string `mapstructure:"siteurl"`
    ProjectID               string `mapstructure:"projectid"`
    Environment             string `mapstructure:"environment"`
    SecretPath              string `mapstructure:"secretpath"`
    ClientID                string `mapstructure:"clientid"`
    ClientSecret            string `mapstructure:"clientsecret"`
    ClientSecretEnv         string `mapstructure:"clientsecretenv"`
    ApifyToken              string `mapstructure:"apifytoken"`
    ApifyTokenEnv           string `mapstructure:"apifytokenenv"`
    GeminiToken             string `mapstructure:"geminitoken"`
    GeminiTokenEnv          string `mapstructure:"geminitokenenv"`
    ApifyActorID            string `mapstructure:"apifyactorid"`
    ApifyBaseURL             string `mapstructure:"apifybaseurl"`
}
```

Add `[infisical]` and provider reference fields to `config.example.toml`. Add environment overrides for non-secret operational values and explicitly named secret environment variables. Keep the local `config.toml` ignored and never add real values to the example file.

- [ ] **Step 4: Run focused tests and verify pass**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./config -run 'Test(Infisical|SocialMediaAnalysis)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/setting.go config/setting_test.go config.example.toml
git commit -m "feat: add dual-source provider configuration"
```

### Task 2: Implement centralized secret resolution

**Files:**
- Create: `service/secret_resolver.go`
- Create: `service/secret_resolver_test.go`
- Modify: `config/setting.go` only if resolver-facing defaults are needed

**Interfaces:**
- `type SecretResolver interface { Resolve(ctx context.Context, name string) (string, error) }`
- `NewSecretResolver(setting config.InfisicalSetting, client *http.Client) SecretResolver`
- Resolution order: Infisical value, explicit configured environment override, TOML fallback.

- [ ] **Step 1: Write failing resolver tests**

Use an `httptest.Server` to model Universal Auth login and raw secret retrieval. Test successful Infisical resolution, TOML fallback when disabled, environment override, missing required values, non-2xx responses, and that errors do not contain secret values.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestSecretResolver' -count=1`

Expected: FAIL because the resolver does not exist.

- [ ] **Step 3: Implement Universal Auth resolution**

Implement the Infisical REST calls:

```text
POST {siteurl}/api/v1/auth/universal-auth/login
{ "clientId": "...", "clientSecret": "..." }

GET {siteurl}/api/v3/secrets/raw?workspaceId={projectid}&environment={environment}&secretPath={secretpath}
Authorization: Bearer {accessToken}
```

Parse only the requested secret names, enforce response size limits, redact values from errors, and cache the access token for its server-provided expiry. Do not log request headers, bodies, or returned secret values.

- [ ] **Step 4: Run resolver tests and verify pass**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestSecretResolver' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/secret_resolver.go service/secret_resolver_test.go
git commit -m "feat: resolve provider secrets from infisical or toml"
```

### Task 3: Expand normalized source data and generic metadata extraction

**Files:**
- Modify: `service/social_media_analysis_types.go`
- Modify: `service/social_media_analysis_source.go`
- Modify: `service/social_media_analysis_source_test.go`

**Interfaces:**
- Extend `NormalizedSocialPost` with bounded `Title`, `Description`, `Hashtags`, `ImageURLs`, `VideoURLs`, and `Metadata map[string]string`.
- Keep `SourceAdapter.Fetch(context.Context, *url.URL) (NormalizedSocialPost, error)` unchanged for callers.

- [ ] **Step 1: Write failing generic extraction tests**

Serve HTML containing `<title>`, description meta tags, `og:image`, `og:video`, JSON-LD/image metadata, and duplicate hashtags. Assert normalized URLs, lowercased deduplicated hashtags, and bounded text. Add a test that malformed HTML does not exceed the source limit.

- [ ] **Step 2: Run focused source tests and verify failure**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test(GenericSource|Extract)' -count=1`

Expected: FAIL because the normalized fields and extraction logic are absent.

- [ ] **Step 3: Implement bounded HTML extraction**

Parse the already bounded response body with the existing HTML tooling. Prefer canonical URL, `og:title`, `og:description`, `description`, `og:image`, `twitter:image`, `og:video`, and `twitter:player:stream`. Extract visible `#tag` tokens and preserve the raw body only within the configured limit. Resolve relative media URLs against the final response URL and deduplicate them.

- [ ] **Step 4: Run source tests and verify pass**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test(GenericSource|Extract)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/social_media_analysis_types.go service/social_media_analysis_source.go service/social_media_analysis_source_test.go
git commit -m "feat: extract normalized metadata from generic sources"
```

### Task 4: Add Apify Instagram and Threads source adapter

**Files:**
- Create: `service/social_media_analysis_apify.go`
- Create: `service/social_media_analysis_apify_test.go`
- Modify: `service/social_media_analysis_source.go`

**Interfaces:**
- `type ApifyClient interface { Run(ctx context.Context, actorID string, input map[string]any) ([]byte, error) }`
- `NewApifyClient(baseURL string, tokenResolver SecretResolver, httpClient *http.Client) ApifyClient`
- `NewSourceAdapterRegistry(...)` routes exact Instagram/Threads hosts to Apify and all others to generic.

- [ ] **Step 1: Write failing Apify client and routing tests**

Use `httptest.Server` to assert the token is sent only as an authorization/query credential to Apify, the configured actor ID is used, the submitted input contains the canonical URL, and the client rejects non-2xx/oversized/malformed responses. Assert Instagram and Threads never invoke the generic adapter when Apify fails.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test(Apify|SourceAdapter)' -count=1`

Expected: FAIL because the Apify client and routing are absent.

- [ ] **Step 3: Implement the Apify boundary**

Submit a configurable actor run through the Apify REST API, poll the run until terminal state with bounded attempts and timeout, then fetch the dataset/result payload. Map common Apify fields into `NormalizedSocialPost`, including text, canonical URL, hashtags, image URLs, and video URLs. Keep the raw provider payload out of prompts and persistence after normalization.

- [ ] **Step 4: Wire adapter selection into the worker**

Construct the registry from configuration at worker startup. Resolve the Apify token only when an Instagram or Threads URL is processed. Return a source-fetch error if the token, actor ID, or Apify result is invalid; do not silently switch adapters.

- [ ] **Step 5: Run Apify/source tests and verify pass**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test(Apify|SourceAdapter)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/social_media_analysis_apify.go service/social_media_analysis_apify_test.go service/social_media_analysis_source.go
git commit -m "feat: route instagram and threads through apify"
```

### Task 5: Pass normalized media and hashtags to the agent

**Files:**
- Modify: `service/social_media_analysis_worker.go`
- Modify: `service/social_media_analysis_worker_test.go`
- Modify: `docs/social-media-analysis.md`

**Interfaces:**
- The existing runner prompt/schema remains strict JSON.
- `buildSocialMediaAnalysisPrompt` consumes the expanded `NormalizedSocialPost` and includes bounded extracted metadata/media references.

- [ ] **Step 1: Write failing worker tests**

Assert the prompt includes extracted hashtags, image URLs, video URLs, title, and description; assert raw source content remains bounded; assert the worker records `source_fetched` before runner submission; and assert a source adapter error prevents runner submission.

- [ ] **Step 2: Run focused worker tests and verify failure**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test(SocialMediaAnalysis|BuildSocialMediaAnalysisPrompt)' -count=1`

Expected: FAIL because the prompt and worker currently only receive the raw source body.

- [ ] **Step 3: Implement prompt and worker integration**

Add clearly labeled normalized source sections to the prompt. Tell the agent to prefer supplied canonical media references, preserve source hashtags as candidates, and only invent tags when supported by source content. Keep credentials and provider payloads excluded.

- [ ] **Step 4: Run focused and full tests**

Run: `GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'Test(SocialMediaAnalysis|BuildSocialMediaAnalysisPrompt)' -count=1`

Then run: `GOCACHE=/tmp/roroadmap-go-cache go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/social_media_analysis_worker.go service/social_media_analysis_worker_test.go docs/social-media-analysis.md
git commit -m "feat: provide normalized scraped media to analysis agent"
```

### Task 6: Verify the complete scraping phase and push the PR

**Files:**
- Modify: `docs/social-media-analysis.md` if operational commands need updating
- Test: existing service/config test suites

- [ ] **Step 1: Add an end-to-end fake-provider test**

Run the analysis job with fake generic, Apify, secret, runner, and marker dependencies. Assert generic URLs and Instagram/Threads URLs produce normalized source data, source acquisition precedes runner submission, and no Google Maps call occurs.

- [ ] **Step 2: Run verification**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./...
GOCACHE=/tmp/roroadmap-go-cache go vet ./...
```

Expected: both commands pass.

- [ ] **Step 3: Push to the existing PR**

```bash
git push origin feat/social-media-analysis-metadata-images
gh pr checks 95
```

Expected: PR #95 contains the scraping-phase implementation and all checks pass.

