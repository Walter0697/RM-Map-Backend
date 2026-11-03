package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
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
	return NormalizedSocialPost{
		SourceURL: sourceURL.String(),
		FinalURL:  response.Request.URL.String(),
		RawBody:   string(body),
	}, nil
}
