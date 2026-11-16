# Google Maps Location Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (recommended) or superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Verify agent-extracted social-media locations with Google Places and invoke a second wcodex agent only when the Google evidence is ambiguous.

**Architecture:** Keep the existing wcodex analysis as Agent 1. After Agent 1 returns a location, the backend optionally queries Google Places Text Search (New), classifies the result deterministically, and either creates the marker or submits Agent 2 with the original post, Agent 1 result, and Google candidates. When Google is disabled or unavailable by configuration, the existing Agent 1-only behavior remains active.

**Tech Stack:** Go, GORM, Viper/TOML, Infisical Universal Auth, `net/http`, Google Places API (New), wcodex-runner, Go test with `httptest`.

## Global Constraints

- The public API remains `POST /integration/social-media/analyses` with `{"url":"..."}` and the existing polling endpoint.
- Google verification is backend-owned and must never expose the Google API key to the runner or persist it in job metadata.
- Agent 2 is conditional; a clear Google match must not incur a second runner call.
- Google is optional. Missing configuration must preserve current behavior rather than fail existing social-analysis jobs.
- Unit tests must not require network access to Google, Infisical, Apify, or wcodex-runner.
- Provider responses must be size-bounded, timeout-bounded, and validated before use.
- Secret precedence remains Infisical, explicitly configured environment override, then TOML fallback.
- Jev is out of scope for this implementation.

---

### Task 1: Add Google Maps configuration and secret resolution

**Files:**
- Modify: `config/setting.go`
- Modify: `config.example.toml`
- Modify: `service/secret_resolver.go`
- Test: `config/setting_test.go`
- Test: `service/secret_resolver_test.go`

**Interfaces:**
- Add `GoogleMapsSetting` with `Enable`, `BaseURL`, `TimeoutMS`, and `MaxResults` fields.
- Resolve `GOOGLE_MAPS_API_KEY` through the existing `SecretResolver`.
- Support an empty TOML fallback field for local/open-source deployments without committing a real key.

- [ ] **Step 1: Write failing configuration tests**

Add tests asserting that `[googlemaps]` values decode into `config.Data.GoogleMaps`, with defaults of `enable=false`, `baseurl=https://places.googleapis.com`, `timeoutms=5000`, and `maxresults=5` applied by configuration initialization when omitted.

- [ ] **Step 2: Run the focused configuration tests**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./config -run 'TestGoogleMaps' -count=1
```

Expected: FAIL because the setting and defaults do not exist.

- [ ] **Step 3: Write the failing secret-resolution test**

Add a resolver test that configures `GOOGLE_MAPS_API_KEY` in the fake Infisical response and verifies `Resolve(context.Background(), "GOOGLE_MAPS_API_KEY")` returns it. Add a fallback test for the TOML value when Infisical is disabled.

- [ ] **Step 4: Implement configuration and resolver mapping**

Add the Google Maps setting to `config.Config`, add the example TOML section:

```toml
[googlemaps]
enable=false
baseurl="https://places.googleapis.com"
timeoutms=5000
maxresults=5
apikey=""
```

Extend the resolver’s explicit environment/TOML mapping for `GOOGLE_MAPS_API_KEY` without logging or serializing the value. Keep Infisical lookup generic so the same key is available from `/roroadmap` in `dev`.

- [ ] **Step 5: Run focused tests**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./config -run 'TestGoogleMaps' -count=1
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestSecretResolver.*GoogleMaps|TestSecretResolver' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add config/setting.go config/setting_test.go config.example.toml service/secret_resolver.go service/secret_resolver_test.go
git commit -m "feat: add Google Maps configuration"
```

### Task 2: Implement the Google Places provider

**Files:**
- Create: `service/google_places.go`
- Create: `service/google_places_test.go`

**Interfaces:**

```go
type GooglePlacesProvider interface {
    Search(ctx context.Context, query string) ([]GooglePlaceCandidate, error)
}

type GooglePlaceCandidate struct {
    PlaceID          string
    Name             string
    Address          string
    Latitude         float64
    Longitude        float64
    Types            []string
    GoogleMapsURI    string
}
```

- [ ] **Step 1: Write failing request and response tests**

Use `httptest.Server` to assert that `Search` sends a POST to `/v1/places:searchText`, includes `textQuery`, sends `X-Goog-Api-Key`, sends the restricted field mask, and parses `places[].id`, `displayName.text`, `formattedAddress`, `location`, `types`, and `googleMapsUri`.

- [ ] **Step 2: Run the provider tests and verify failure**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestGooglePlaces' -count=1
```

Expected: FAIL because the provider does not exist.

- [ ] **Step 3: Implement the provider**

Implement a bounded HTTP client using the configured base URL and timeout. Resolve `GOOGLE_MAPS_API_KEY` immediately before the request. Send the field mask:

```text
places.id,places.displayName,places.formattedAddress,places.location,places.types,places.googleMapsUri
```

Set `pageSize` to the configured maximum. Return a typed error for missing configuration, non-2xx responses, empty responses, malformed JSON, or missing required candidate fields. Never include the key in an error string.

- [ ] **Step 4: Run provider tests**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestGooglePlaces' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/google_places.go service/google_places_test.go
git commit -m "feat: add Google Places search provider"
```

### Task 3: Add deterministic candidate comparison

**Files:**
- Create: `service/location_verification.go`
- Create: `service/location_verification_test.go`

**Interfaces:**

```go
type LocationVerificationStatus string

const (
    LocationMatch     LocationVerificationStatus = "match"
    LocationAmbiguous LocationVerificationStatus = "ambiguous"
    LocationNoMatch   LocationVerificationStatus = "no_match"
)

type LocationVerification struct {
    Status       LocationVerificationStatus
    Candidate    *GooglePlaceCandidate
    Candidates   []GooglePlaceCandidate
    Reason       string
}

func VerifyLocation(agent LocationCandidate, places []GooglePlaceCandidate) LocationVerification
```

- [ ] **Step 1: Write failing comparison tests**

Cover these exact cases:

1. One same-name, same-city, nearby result returns `match`.
2. Two plausible results returns `ambiguous`.
3. No related result returns `no_match`.
4. A large coordinate mismatch rejects an otherwise similar name.
5. Empty Google results return `no_match` without panicking.

- [ ] **Step 2: Run comparison tests and verify failure**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestVerifyLocation' -count=1
```

Expected: FAIL because the comparison types and function do not exist.

- [ ] **Step 3: Implement normalized scoring and thresholds**

Normalize case, punctuation, and whitespace into comparable tokens. Score name overlap, address overlap, and coordinate distance using a small Haversine helper. Return `match` only when one candidate clearly leads and passes the distance/name checks; return `ambiguous` when multiple candidates remain plausible; return `no_match` otherwise.

- [ ] **Step 4: Run comparison tests**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestVerifyLocation' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/location_verification.go service/location_verification_test.go
git commit -m "feat: compare agent locations with Google places"
```

### Task 4: Add conditional Agent 2 adjudication to the worker

**Files:**
- Modify: `service/social_media_analysis_worker.go`
- Modify: `service/social_media_analysis_types.go`
- Test: `service/social_media_analysis_worker_test.go`

**Interfaces:**
- Keep `ProcessSocialMediaAnalysisJob` and `reconcileRunnerJob` as the worker entry points.
- Add an injectable Google Places provider factory for unit tests.
- Add a comparison-agent prompt builder and schema that accepts Agent 1 output plus Google candidates.

- [ ] **Step 1: Write failing worker tests**

Add tests verifying:

1. Google disabled creates a marker through the existing Agent 1 path.
2. Google returns one strong match and Agent 2 is not submitted.
3. Google returns ambiguous candidates and Agent 2 is submitted with both candidate sets in its prompt.
4. Agent 2 returns `needs_input`, leaving `CreatedMarkerID` empty.
5. Agent 2 returns `create_marker`, using the final validated location and marker type.
6. A missing Google key follows the current Agent 1-only path.

- [ ] **Step 2: Run the focused worker tests and verify failure**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestSocialMediaAnalysis.*Google|Test.*ComparisonAgent' -count=1
```

Expected: FAIL because the worker has no Google verification or second-agent stage.

- [ ] **Step 3: Implement first-stage verification**

After Agent 1 completes, preserve the decoded result in the job’s existing flow, query Google only when enabled and a location candidate exists, and call `VerifyLocation`. For `match`, replace the marker location with the verified Google coordinates/address and proceed to marker creation. For disabled/missing-key behavior, retain the current direct path.

- [ ] **Step 4: Implement second-agent submission and reconciliation**

For `ambiguous` or `no_match`, submit a second runner job with an explicit prompt containing:

- normalized post content and media references;
- Agent 1’s complete result;
- all Google candidates and their addresses/coordinates;
- the requirement to return `create_marker` only when one location is reliable.

Set a distinct job stage such as `location_comparison_submitted` and reuse the persisted runner job ID. On completion, decode and validate Agent 2’s result, then create the marker only for `create_marker` with a valid location. Keep the final result in `result_json`.

- [ ] **Step 5: Run worker tests**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./service -run 'TestSocialMediaAnalysis.*Google|Test.*ComparisonAgent' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/social_media_analysis_worker.go service/social_media_analysis_types.go service/social_media_analysis_worker_test.go
git commit -m "feat: verify social locations with Google Places"
```

### Task 5: Document setup and run the full verification suite

**Files:**
- Modify: `docs/social-media-analysis.md`
- Modify: `config.example.toml` if Task 1 did not include the final setup text
- Test: repository-wide tests and vet

- [ ] **Step 1: Document Infisical setup**

Document the exact development setup:

```text
Infisical project: roroadmap
Environment: dev
Secret path: /roroadmap
Secret: GOOGLE_MAPS_API_KEY
```

Document that Google Cloud must have Places API (New) enabled, billing/quotas configured, and the key restricted to the required API. Document the optional TOML fallback without a real key.

- [ ] **Step 2: Run the full suite**

Run:

```bash
GOCACHE=/tmp/roroadmap-go-cache go test ./...
GOCACHE=/tmp/roroadmap-go-cache go vet ./...
git diff --check
```

Expected: all tests pass, vet produces no diagnostics, and the diff has no whitespace errors.

- [ ] **Step 3: Run a live development verification**

After `GOOGLE_MAPS_API_KEY` is present in Infisical, restart the local stack and submit an Instagram or website URL whose caption includes a recognizable venue. Verify the job stages, Google candidate comparison, marker coordinates, image path, marker type, and type icon. Do not print or persist the API key.

- [ ] **Step 4: Commit documentation and verification changes**

```bash
git add docs/social-media-analysis.md config.example.toml
git commit -m "docs: document Google Maps location verification"
```
