package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"mapmarker/backend/config"
)

type ApifyClient interface {
	Fetch(context.Context, *url.URL) (NormalizedSocialPost, error)
}

type apifyClient struct {
	baseURL         string
	actorID         string
	actorSecretName string
	inputMode       string
	secrets         SecretResolver
	http            *http.Client
	maxBytes        int64
}

func NewApifyClient(baseURL, actorID string, secrets SecretResolver, client *http.Client) SourceAdapter {
	return newApifyClient(baseURL, actorID, "APIFY_ACTOR_ID", "legacy", secrets, client)
}

func newApifyClient(baseURL, actorID, actorSecretName, inputMode string, secrets SecretResolver, client *http.Client) SourceAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return apifyClient{
		baseURL:         strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		actorID:         strings.TrimSpace(actorID),
		actorSecretName: strings.TrimSpace(actorSecretName),
		inputMode:       strings.TrimSpace(inputMode),
		secrets:         secrets,
		http:            client,
		maxBytes:        5 * 1024 * 1024,
	}
}

func (client apifyClient) Name() string { return "apify" }

func (client apifyClient) Fetch(ctx context.Context, sourceURL *url.URL) (NormalizedSocialPost, error) {
	if sourceURL == nil || strings.TrimSpace(sourceURL.String()) == "" {
		return NormalizedSocialPost{}, fmt.Errorf("Apify source URL is required")
	}
	if client.baseURL == "" {
		return NormalizedSocialPost{}, fmt.Errorf("Apify configuration is incomplete")
	}
	if client.secrets == nil {
		return NormalizedSocialPost{}, fmt.Errorf("Apify secret resolver is required")
	}
	token, err := client.secrets.Resolve(ctx, "APIFY_TOKEN")
	if err != nil {
		return NormalizedSocialPost{}, err
	}
	actorID := client.actorID
	if actorID == "" {
		secretName := client.actorSecretName
		if secretName == "" {
			secretName = "APIFY_ACTOR_ID"
		}
		actorID, err = client.secrets.Resolve(ctx, secretName)
		if (err != nil || strings.TrimSpace(actorID) == "") && secretName != "APIFY_ACTOR_ID" {
			actorID, err = client.secrets.Resolve(ctx, "APIFY_ACTOR_ID")
		}
		if err != nil {
			return NormalizedSocialPost{}, err
		}
	}
	if strings.TrimSpace(actorID) == "" {
		return NormalizedSocialPost{}, fmt.Errorf("Apify actor ID is not configured")
	}
	input, err := client.input(sourceURL)
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("encode Apify input: %w", err)
	}
	actorPath := path.Join("/v2/acts", actorID, "runs")
	runPayload, err := client.doJSON(ctx, http.MethodPost, actorPath, token, bytes.NewReader(input))
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("start Apify actor: %w", err)
	}
	var run struct {
		Data struct {
			ID               string `json:"id"`
			DefaultDatasetID string `json:"defaultDatasetId"`
			Status           string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(runPayload, &run); err != nil || run.Data.ID == "" {
		return NormalizedSocialPost{}, fmt.Errorf("decode Apify actor response")
	}
	datasetID := run.Data.DefaultDatasetID
	if datasetID == "" {
		datasetID, err = client.waitForDataset(ctx, run.Data.ID, token)
		if err != nil {
			return NormalizedSocialPost{}, err
		}
	}
	items, err := client.doJSON(ctx, http.MethodGet, path.Join("/v2/datasets", datasetID, "items"), token, nil)
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("fetch Apify dataset: %w", err)
	}
	return normalizeApifyDataset(sourceURL, items, client.maxBytes)
}

func (client apifyClient) input(sourceURL *url.URL) ([]byte, error) {
	var input map[string]any
	switch client.inputMode {
	case "instagram":
		input = map[string]any{
			"directUrls":   []string{sourceURL.String()},
			"resultsType":  "posts",
			"resultsLimit": 1,
		}
	case "threads":
		input = map[string]any{
			"post_urls": []string{sourceURL.String()},
		}
	default:
		input = map[string]any{"url": sourceURL.String()}
	}
	return json.Marshal(input)
}

func (client apifyClient) waitForDataset(ctx context.Context, runID, token string) (string, error) {
	for attempt := 0; attempt < 30; attempt++ {
		payload, err := client.doJSON(ctx, http.MethodGet, path.Join("/v2/actor-runs", runID), token, nil)
		if err != nil {
			return "", fmt.Errorf("poll Apify actor: %w", err)
		}
		var status struct {
			Data struct {
				Status           string `json:"status"`
				DefaultDatasetID string `json:"defaultDatasetId"`
			} `json:"data"`
		}
		if err := json.Unmarshal(payload, &status); err != nil {
			return "", fmt.Errorf("decode Apify actor status")
		}
		if status.Data.DefaultDatasetID != "" {
			return status.Data.DefaultDatasetID, nil
		}
		if status.Data.Status == "FAILED" || status.Data.Status == "ABORTED" || status.Data.Status == "TIMED-OUT" {
			return "", fmt.Errorf("Apify actor ended with status %s", status.Data.Status)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return "", fmt.Errorf("Apify actor did not finish within polling limit")
}

func (client apifyClient) doJSON(ctx context.Context, method, endpoint, token string, body io.Reader) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("create Apify request: %w", err)
	}
	query := request.URL.Query()
	query.Set("token", token)
	request.URL.RawQuery = query.Encode()
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Apify returned HTTP %d", response.StatusCode)
	}
	result, err := io.ReadAll(io.LimitReader(response.Body, client.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(result)) > client.maxBytes {
		return nil, fmt.Errorf("Apify response exceeds size limit")
	}
	return result, nil
}

func normalizeApifyDataset(sourceURL *url.URL, payload []byte, maxBytes int64) (NormalizedSocialPost, error) {
	if int64(len(payload)) > maxBytes {
		return NormalizedSocialPost{}, fmt.Errorf("Apify dataset exceeds size limit")
	}
	var items []map[string]any
	if err := json.Unmarshal(payload, &items); err != nil || len(items) == 0 {
		return NormalizedSocialPost{}, fmt.Errorf("decode Apify dataset")
	}
	item := items[0]
	post := NormalizedSocialPost{SourceURL: sourceURL.String(), FinalURL: sourceURL.String(), Metadata: map[string]string{}}
	post.Title = firstJSONField(item, "title", "name")
	post.Description = firstJSONField(item, "description", "caption", "text")
	caption := firstJSONField(item, "caption", "text", "description")
	if canonical := firstJSONField(item, "url", "canonicalUrl", "canonical_url"); canonical != "" {
		post.FinalURL = canonical
	}
	for _, value := range []string{firstJSONField(item, "displayUrl", "imageUrl", "image_url", "thumbnailUrl", "thumbnail_url")} {
		appendResolvedURL(&post.ImageURLs, value, sourceURL)
	}
	for _, value := range []string{firstJSONField(item, "videoUrl", "video_url")} {
		appendResolvedURL(&post.VideoURLs, value, sourceURL)
	}
	if caption != "" {
		for _, match := range sourceHashtagPattern.FindAllStringSubmatch(caption, 32) {
			if len(match) > 1 {
				value := strings.ToLower(match[1])
				if !containsString(post.Hashtags, value) {
					post.Hashtags = append(post.Hashtags, value)
				}
			}
		}
	}
	post.RawBody = post.Description
	if len(post.Metadata) == 0 {
		post.Metadata = nil
	}
	return post, nil
}

func firstJSONField(item map[string]any, names ...string) string {
	for _, name := range names {
		if value, ok := item[name].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func SelectConfiguredSourceAdapter(rawURL, actorID string, secrets SecretResolver, client *http.Client) SourceAdapter {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return sourceAdapter{name: "generic", fetch: genericSourceFetcher}
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "instagram.com" || strings.HasSuffix(host, ".instagram.com") {
		return namedSourceAdapter{name: "instagram", adapter: newApifyClient(configuredApifyBaseURL(), "", "APIFY_INSTAGRAM_ACTOR_ID", "instagram", secrets, client)}
	}
	if host == "threads.net" || strings.HasSuffix(host, ".threads.net") || host == "threads.com" || strings.HasSuffix(host, ".threads.com") {
		return namedSourceAdapter{name: "threads", adapter: newApifyClient(configuredApifyBaseURL(), actorID, "APIFY_THREADS_ACTOR_ID", "threads", secrets, client)}
	}
	return sourceAdapter{name: "generic", fetch: genericSourceFetcher}
}

type namedSourceAdapter struct {
	name    string
	adapter SourceAdapter
}

func (adapter namedSourceAdapter) Name() string { return adapter.name }

func (adapter namedSourceAdapter) Fetch(ctx context.Context, sourceURL *url.URL) (NormalizedSocialPost, error) {
	return adapter.adapter.Fetch(ctx, sourceURL)
}

func configuredApifyBaseURL() string {
	if value := strings.TrimSpace(config.Data.Infisical.ApifyBaseURL); value != "" {
		return value
	}
	return "https://api.apify.com"
}
