package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"mapmarker/backend/config"
)

const maxSecretResponseBytes = 1024 * 1024

type SecretResolver interface {
	Resolve(context.Context, string) (string, error)
}

type infisicalSecretResolver struct {
	setting config.InfisicalSetting
	client  *http.Client

	mu          sync.Mutex
	accessToken string
	tokenUntil  time.Time
	secrets     map[string]string
}

func NewSecretResolver(setting config.InfisicalSetting, client *http.Client) SecretResolver {
	if client == nil {
		client = http.DefaultClient
	}
	return &infisicalSecretResolver{setting: setting, client: client, secrets: make(map[string]string)}
}

func (resolver *infisicalSecretResolver) Resolve(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("secret name is required")
	}

	if resolver.setting.Enable {
		value, err := resolver.resolveInfisical(ctx, name)
		if err == nil && strings.TrimSpace(value) != "" {
			return value, nil
		}
		if err != nil {
			return "", err
		}
	}

	if envName := resolver.environmentName(name); envName != "" {
		if value := os.Getenv(envName); strings.TrimSpace(value) != "" {
			return value, nil
		}
	}
	if value := resolver.tomlValue(name); strings.TrimSpace(value) != "" {
		return value, nil
	}
	return "", fmt.Errorf("secret %s is not configured", name)
}

func (resolver *infisicalSecretResolver) resolveInfisical(ctx context.Context, name string) (string, error) {
	resolver.mu.Lock()
	if value, ok := resolver.secrets[name]; ok {
		resolver.mu.Unlock()
		return value, nil
	}
	resolver.mu.Unlock()

	if strings.TrimSpace(resolver.setting.SiteURL) == "" ||
		strings.TrimSpace(resolver.setting.ProjectID) == "" ||
		strings.TrimSpace(resolver.setting.Environment) == "" ||
		strings.TrimSpace(resolver.setting.ClientID) == "" {
		return "", fmt.Errorf("Infisical configuration is incomplete for secret %s", name)
	}

	token, err := resolver.getAccessToken(ctx)
	if err != nil {
		return "", err
	}

	endpoint, err := url.Parse(strings.TrimRight(resolver.setting.SiteURL, "/") + "/api/v3/secrets/raw")
	if err != nil {
		return "", fmt.Errorf("build Infisical secrets URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("workspaceId", resolver.setting.ProjectID)
	query.Set("environment", resolver.setting.Environment)
	query.Set("secretPath", resolver.setting.SecretPath)
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create Infisical secrets request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := resolver.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch Infisical secret %s: %w", name, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("fetch Infisical secret %s returned HTTP %d", name, response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxSecretResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("read Infisical secret response: %w", err)
	}
	if int64(len(body)) > maxSecretResponseBytes {
		return "", fmt.Errorf("Infisical secret response exceeds size limit")
	}
	var payload struct {
		Secrets []struct {
			Key   string `json:"secretKey"`
			Value string `json:"secretValue"`
		} `json:"secrets"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode Infisical secret response: %w", err)
	}
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	for _, secret := range payload.Secrets {
		resolver.secrets[secret.Key] = secret.Value
	}
	return resolver.secrets[name], nil
}

func (resolver *infisicalSecretResolver) getAccessToken(ctx context.Context) (string, error) {
	resolver.mu.Lock()
	if resolver.accessToken != "" && time.Now().Before(resolver.tokenUntil) {
		token := resolver.accessToken
		resolver.mu.Unlock()
		return token, nil
	}
	resolver.mu.Unlock()

	clientSecret := resolver.setting.ClientSecret
	if resolver.setting.ClientSecretEnv != "" {
		if value := os.Getenv(resolver.setting.ClientSecretEnv); strings.TrimSpace(value) != "" {
			clientSecret = value
		}
	}
	if clientSecret == "" {
		return "", fmt.Errorf("Infisical client secret is not configured")
	}
	payload, err := json.Marshal(map[string]string{
		"clientId":     resolver.setting.ClientID,
		"clientSecret": clientSecret,
	})
	if err != nil {
		return "", fmt.Errorf("encode Infisical login request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(resolver.setting.SiteURL, "/")+"/api/v1/auth/universal-auth/login", strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("create Infisical login request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := resolver.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("login to Infisical: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("login to Infisical returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSecretResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read Infisical login response: %w", err)
	}
	var result struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int    `json:"expiresIn"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" {
		return "", fmt.Errorf("decode Infisical login response")
	}
	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	resolver.mu.Lock()
	resolver.accessToken = result.AccessToken
	resolver.tokenUntil = time.Now().Add(ttl - time.Minute)
	resolver.mu.Unlock()
	return result.AccessToken, nil
}

func (resolver *infisicalSecretResolver) environmentName(name string) string {
	switch name {
	case "APIFY_TOKEN":
		return resolver.setting.ApifyTokenEnv
	case "GEMINI_TOKEN":
		return resolver.setting.GeminiTokenEnv
	default:
		return ""
	}
}

func (resolver *infisicalSecretResolver) tomlValue(name string) string {
	switch name {
	case "APIFY_TOKEN":
		return resolver.setting.ApifyToken
	case "GEMINI_TOKEN":
		return resolver.setting.GeminiToken
	case "APIFY_ACTOR_ID":
		return resolver.setting.ApifyActorID
	default:
		return ""
	}
}
