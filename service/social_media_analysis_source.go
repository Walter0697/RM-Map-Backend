package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	ErrInvalidSourceURL         = errors.New("invalid source URL")
	ErrBlockedSourceURL         = errors.New("blocked source URL")
	ErrSourceAdapterUnavailable = errors.New("source adapter unavailable")
)

type SourceAdapter interface {
	Name() string
	Fetch(context.Context, *url.URL) (NormalizedSocialPost, error)
}

type sourceAdapter struct {
	name  string
	fetch func(context.Context, *url.URL) (NormalizedSocialPost, error)
}

func (adapter sourceAdapter) Name() string {
	return adapter.name
}

func (adapter sourceAdapter) Fetch(ctx context.Context, sourceURL *url.URL) (NormalizedSocialPost, error) {
	if adapter.fetch == nil {
		return NormalizedSocialPost{}, fmt.Errorf("%w: %s", ErrSourceAdapterUnavailable, adapter.name)
	}
	return adapter.fetch(ctx, sourceURL)
}

func ValidatePublicSourceURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: URL is required", ErrInvalidSourceURL)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%w: malformed URL", ErrInvalidSourceURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: scheme %q is not allowed", ErrInvalidSourceURL, parsed.Scheme)
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("%w: embedded credentials are not allowed", ErrInvalidSourceURL)
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return nil, fmt.Errorf("%w: host is required", ErrInvalidSourceURL)
	}
	if isBlockedSourceHost(host) {
		return nil, fmt.Errorf("%w: host %q is not public", ErrBlockedSourceURL, host)
	}
	return parsed, nil
}

func isBlockedSourceHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func SelectSourceAdapter(rawURL string) SourceAdapter {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return sourceAdapter{name: "generic", fetch: genericSourceFetcher}
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "instagram.com" || strings.HasSuffix(host, ".instagram.com"):
		return sourceAdapter{name: "instagram", fetch: genericSourceFetcher}
	case host == "threads.net" || strings.HasSuffix(host, ".threads.net") || host == "threads.com" || strings.HasSuffix(host, ".threads.com"):
		return sourceAdapter{name: "threads", fetch: genericSourceFetcher}
	default:
		return sourceAdapter{name: "generic", fetch: genericSourceFetcher}
	}
}

func genericSourceFetcher(ctx context.Context, sourceURL *url.URL) (NormalizedSocialPost, error) {
	return fetchGenericSource(ctx, sourceURL, http.DefaultClient, 5*1024*1024)
}

func fetchGenericSource(ctx context.Context, sourceURL *url.URL, client *http.Client, maxBytes int64) (NormalizedSocialPost, error) {
	if sourceURL == nil {
		return NormalizedSocialPost{}, fmt.Errorf("%w: URL is required", ErrInvalidSourceURL)
	}
	if maxBytes <= 0 {
		return NormalizedSocialPost{}, fmt.Errorf("response size limit must be positive")
	}
	if client == nil {
		client = &http.Client{}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(request *http.Request, _ []*http.Request) error {
		if _, err := ValidatePublicSourceURL(request.URL.String()); err != nil {
			return fmt.Errorf("blocked redirect: %w", err)
		}
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL.String(), nil)
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("create source request: %w", err)
	}
	response, err := clientCopy.Do(request)
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("fetch source: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return NormalizedSocialPost{}, fmt.Errorf("source returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return NormalizedSocialPost{}, fmt.Errorf("read source response: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return NormalizedSocialPost{}, fmt.Errorf("source response too large: limit is %d bytes", maxBytes)
	}
	finalURL := response.Request.URL.String()
	post := NormalizedSocialPost{
		SourceURL: sourceURL.String(),
		FinalURL:  finalURL,
		RawBody:   string(body),
	}
	if contentType := response.Header.Get("Content-Type"); strings.Contains(strings.ToLower(contentType), "html") || contentType == "" {
		extractGenericMetadata(&post, body, response.Request.URL)
	}
	return post, nil
}

var sourceHashtagPattern = regexp.MustCompile(`(?i)#([\p{L}\p{N}_-]+)`)

func extractGenericMetadata(post *NormalizedSocialPost, body []byte, baseURL *url.URL) {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return
	}
	post.Title = strings.TrimSpace(document.Find("title").First().Text())
	post.Description = firstMetaContent(document, "name", "description")
	if post.Description == "" {
		post.Description = firstMetaContent(document, "property", "og:description")
	}
	post.Metadata = map[string]string{}
	for _, item := range []struct {
		key      string
		selector string
	}{
		{key: "canonical_url", selector: `link[rel="canonical"]`},
		{key: "og_title", selector: `meta[property="og:title"]`},
		{key: "og_description", selector: `meta[property="og:description"]`},
	} {
		value := ""
		if item.key == "canonical_url" {
			value, _ = document.Find(item.selector).First().Attr("href")
		} else {
			value, _ = document.Find(item.selector).First().Attr("content")
		}
		if strings.TrimSpace(value) != "" {
			post.Metadata[item.key] = strings.TrimSpace(value)
		}
	}
	if canonical := post.Metadata["canonical_url"]; canonical != "" {
		if parsed, err := baseURL.Parse(canonical); err == nil {
			post.FinalURL = parsed.String()
		}
	}
	for _, selector := range []string{`meta[property="og:image"]`, `meta[name="twitter:image"]`, `meta[property="og:image:url"]`} {
		document.Find(selector).Each(func(_ int, selection *goquery.Selection) {
			if value, ok := selection.Attr("content"); ok {
				appendResolvedURL(&post.ImageURLs, value, baseURL)
			}
		})
	}
	for _, selector := range []string{`meta[property="og:video"]`, `meta[property="og:video:url"]`, `meta[name="twitter:player:stream"]`} {
		document.Find(selector).Each(func(_ int, selection *goquery.Selection) {
			if value, ok := selection.Attr("content"); ok {
				appendResolvedURL(&post.VideoURLs, value, baseURL)
			}
		})
	}
	for _, match := range sourceHashtagPattern.FindAllStringSubmatch(string(body), 32) {
		if len(match) > 1 {
			value := strings.ToLower(strings.TrimSpace(match[1]))
			if !containsString(post.Hashtags, value) {
				post.Hashtags = append(post.Hashtags, value)
			}
		}
	}
	if len(post.Metadata) == 0 {
		post.Metadata = nil
	}
}

func firstMetaContent(document *goquery.Document, attribute, value string) string {
	content, _ := document.Find(fmt.Sprintf(`meta[%s="%s"]`, attribute, value)).First().Attr("content")
	return strings.TrimSpace(content)
}

func appendResolvedURL(target *[]string, raw string, baseURL *url.URL) {
	parsed, err := baseURL.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return
	}
	for _, existing := range *target {
		if existing == parsed.String() {
			return
		}
	}
	if len(*target) < 16 {
		*target = append(*target, parsed.String())
	}
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
