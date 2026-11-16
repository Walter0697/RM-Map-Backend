package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultInstagramBaseURL = "https://www.instagram.com"
	defaultInstagramDocID   = "9510064595728286"
	instagramUserAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var instagramShortcodePattern = regexp.MustCompile(`(?i)/(?:p|reel|reels|tv)/([A-Za-z0-9_-]+)`)

type instagramGuestClient struct {
	baseURL  string
	docID    string
	http     *http.Client
	maxBytes int64
}

func NewInstagramGuestClient(baseURL, docID string, client *http.Client) SourceAdapter {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultInstagramBaseURL
	}
	if strings.TrimSpace(docID) == "" {
		docID = defaultInstagramDocID
	}
	if client == nil {
		client = http.DefaultClient
	}
	return instagramGuestClient{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		docID:    strings.TrimSpace(docID),
		http:     client,
		maxBytes: sourceMaxBytes(),
	}
}

func (client instagramGuestClient) Name() string { return "instagram-guest" }

func (client instagramGuestClient) Fetch(ctx context.Context, sourceURL *url.URL) (NormalizedSocialPost, error) {
	if sourceURL == nil {
		return NormalizedSocialPost{}, fmt.Errorf("Instagram source URL is required")
	}
	shortcode := instagramShortcode(sourceURL)
	if shortcode == "" {
		return NormalizedSocialPost{}, fmt.Errorf("Instagram URL does not contain a post shortcode")
	}
	if client.baseURL == "" || client.docID == "" {
		return NormalizedSocialPost{}, fmt.Errorf("Instagram Guest configuration is incomplete")
	}

	cookies, err := client.handshake(ctx)
	if err != nil {
		return NormalizedSocialPost{}, err
	}
	variables, _ := json.Marshal(map[string]any{
		"shortcode":               shortcode,
		"fetch_tagged_user_count": nil,
		"hoisted_comment_id":      nil,
		"hoisted_reply_id":        nil,
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/graphql/query", strings.NewReader("variables="+url.QueryEscape(string(variables))+"&doc_id="+url.QueryEscape(client.docID)))
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("create Instagram GraphQL request: %w", err)
	}
	request.Header.Set("User-Agent", instagramUserAgent)
	request.Header.Set("X-CSRFToken", cookies.csrfToken)
	request.Header.Set("Cookie", cookies.header)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.http.Do(request)
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("fetch Instagram GraphQL data: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return NormalizedSocialPost{}, fmt.Errorf("Instagram GraphQL returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, client.maxBytes+1))
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("read Instagram GraphQL response: %w", err)
	}
	if int64(len(body)) > client.maxBytes {
		return NormalizedSocialPost{}, fmt.Errorf("Instagram GraphQL response exceeds size limit")
	}
	return normalizeInstagramGraphQL(sourceURL, body)
}

type instagramSessionCookies struct {
	csrfToken string
	header    string
}

func (client instagramGuestClient) handshake(ctx context.Context) (instagramSessionCookies, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/", nil)
	if err != nil {
		return instagramSessionCookies{}, fmt.Errorf("create Instagram handshake request: %w", err)
	}
	request.Header.Set("User-Agent", instagramUserAgent)
	response, err := client.http.Do(request)
	if err != nil {
		return instagramSessionCookies{}, fmt.Errorf("fetch Instagram handshake: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return instagramSessionCookies{}, fmt.Errorf("Instagram handshake returned HTTP %d", response.StatusCode)
	}
	cookies := response.Cookies()
	if len(cookies) == 0 {
		return instagramSessionCookies{}, fmt.Errorf("Instagram did not return any cookies")
	}
	parts := make([]string, 0, len(cookies))
	csrfToken := ""
	for _, cookie := range cookies {
		parts = append(parts, cookie.Name+"="+cookie.Value)
		if cookie.Name == "csrftoken" {
			csrfToken = cookie.Value
		}
	}
	if csrfToken == "" {
		return instagramSessionCookies{}, fmt.Errorf("Instagram CSRF token not found in cookies")
	}
	return instagramSessionCookies{csrfToken: csrfToken, header: strings.Join(parts, "; ")}, nil
}

func instagramShortcode(sourceURL *url.URL) string {
	if sourceURL == nil {
		return ""
	}
	matches := instagramShortcodePattern.FindStringSubmatch(sourceURL.Path)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func normalizeInstagramGraphQL(sourceURL *url.URL, body []byte) (NormalizedSocialPost, error) {
	var response struct {
		Data struct {
			Media map[string]any `json:"xdt_shortcode_media"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("decode Instagram GraphQL response: %w", err)
	}
	media := response.Data.Media
	if len(media) == 0 {
		return NormalizedSocialPost{}, fmt.Errorf("Instagram response contains no media data")
	}
	post := NormalizedSocialPost{
		SourceURL: sourceURL.String(),
		FinalURL:  sourceURL.String(),
		Metadata:  map[string]string{},
	}
	caption := instagramCaption(media)
	post.Description = caption
	for _, match := range sourceHashtagPattern.FindAllStringSubmatch(caption, 32) {
		if len(match) > 1 && !containsString(post.Hashtags, strings.ToLower(match[1])) {
			post.Hashtags = append(post.Hashtags, strings.ToLower(match[1]))
		}
	}
	for _, item := range instagramMediaItems(media) {
		isVideo, _ := item["is_video"].(bool)
		if isVideo {
			if value, ok := item["video_url"].(string); ok && strings.TrimSpace(value) != "" {
				appendResolvedURL(&post.VideoURLs, value, sourceURL)
			}
			if value, ok := item["display_url"].(string); ok && strings.TrimSpace(value) != "" {
				appendResolvedURL(&post.ImageURLs, value, sourceURL)
			}
			continue
		}
		if value, ok := item["display_url"].(string); ok && strings.TrimSpace(value) != "" {
			appendResolvedURL(&post.ImageURLs, value, sourceURL)
		}
	}
	if owner, ok := media["owner"].(map[string]any); ok {
		instagramMetadataString(post.Metadata, "owner_username", owner["username"])
		instagramMetadataString(post.Metadata, "owner_full_name", owner["full_name"])
		instagramMetadataString(post.Metadata, "is_verified", owner["is_verified"])
	}
	if likes, ok := media["edge_media_preview_like"].(map[string]any); ok {
		instagramMetadataString(post.Metadata, "likes_count", likes["count"])
	}
	if len(post.Metadata) == 0 {
		post.Metadata = nil
	}
	post.RawBody = string(body)
	return post, nil
}

func instagramCaption(media map[string]any) string {
	edgesContainer, _ := media["edge_media_to_caption"].(map[string]any)
	edges, _ := edgesContainer["edges"].([]any)
	if len(edges) == 0 {
		return ""
	}
	first, _ := edges[0].(map[string]any)
	node, _ := first["node"].(map[string]any)
	caption, _ := node["text"].(string)
	return strings.TrimSpace(caption)
}

func instagramMediaItems(media map[string]any) []map[string]any {
	if children, ok := media["edge_sidecar_to_children"].(map[string]any); ok {
		if edges, ok := children["edges"].([]any); ok {
			items := make([]map[string]any, 0, len(edges))
			for _, edge := range edges {
				entry, _ := edge.(map[string]any)
				item, _ := entry["node"].(map[string]any)
				if item != nil {
					items = append(items, item)
				}
			}
			return items
		}
	}
	return []map[string]any{media}
}

func instagramMetadataString(target map[string]string, key string, value any) {
	if value == nil {
		return
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			target[key] = strings.TrimSpace(typed)
		}
	case bool:
		target[key] = strconv.FormatBool(typed)
	case float64:
		target[key] = strconv.FormatFloat(typed, 'f', -1, 64)
	}
}
