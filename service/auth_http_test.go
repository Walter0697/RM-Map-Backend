package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mapmarker/backend/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOIDCStartHandlerRedirectsToProvider(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()

	config.Data.App.Environment = "production"
	config.Data.App.AuthMode = config.AuthModeOIDC
	config.Data.OIDC.Enable = true
	config.Data.OIDC.ClientID = "client-id"
	config.Data.OIDC.ClientSecret = "secret"
	config.Data.OIDC.Issuer = "http://issuer.example"
	config.Data.OIDC.RedirectURL = "http://localhost:1998/auth/oidc/callback"
	config.Data.OIDC.AuthEndpoint = "http://issuer.example/auth"
	config.Data.OIDC.TokenEndpoint = "http://issuer.example/token"
	config.Data.OIDC.Scopes = []string{"openid", "profile"}

	request := httptest.NewRequest(http.MethodGet, "/auth/oidc/start", nil)
	recorder := httptest.NewRecorder()

	OIDCStartHandler(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}
	location := recorder.Header().Get("Location")
	if !strings.HasPrefix(location, config.Data.OIDC.AuthEndpoint) {
		t.Fatalf("expected redirect to auth endpoint, got %s", location)
	}
}

func TestOIDCCallbackHandlerCreatesSessionRedirect(t *testing.T) {
	original := config.Data
	defer func() { config.Data = original }()
	originalUpsert := oidcUpsertUserFn
	defer func() { oidcUpsertUserFn = originalUpsert }()
	originalClient := oidcHTTPClient
	defer func() { oidcHTTPClient = originalClient }()

	oidcHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/token":
				claims := map[string]string{
					"preferred_username": "authentik-user",
				}
				payload, _ := json.Marshal(claims)
				idToken := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
				body := []byte(fmt.Sprintf(`{"access_token":"access-token","id_token":"%s"}`, idToken))
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			case "/userinfo":
				body := []byte(`{"preferred_username":"authentik-user"}`)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
					Header:     make(http.Header),
				}, nil
			}
		}),
	}

	config.Data.App.Environment = "production"
	config.Data.App.AuthMode = config.AuthModeOIDC
	config.Data.OIDC.Enable = true
	config.Data.OIDC.ClientID = "client-id"
	config.Data.OIDC.ClientSecret = "secret"
	config.Data.OIDC.Issuer = "http://provider.example"
	config.Data.OIDC.RedirectURL = "http://localhost:1998/auth/oidc/callback"
	config.Data.OIDC.FrontendRedirectURL = "http://localhost:3000/login"
	config.Data.OIDC.AuthEndpoint = "http://provider.example/auth"
	config.Data.OIDC.TokenEndpoint = "http://provider.example/token"
	config.Data.OIDC.UserInfoEndpoint = "http://provider.example/userinfo"
	config.Data.OIDC.UsernameClaim = "preferred_username"
	config.Data.OIDC.DefaultRole = "user"

	oidcUpsertUserFn = func(username string, defaultRole string) (string, error) {
		if username != "authentik-user" {
			t.Fatalf("unexpected username: %s", username)
		}
		if defaultRole != "user" {
			t.Fatalf("unexpected default role: %s", defaultRole)
		}
		return "jwt-token", nil
	}

	state := "test-state"
	oidcStateStore.Store(state, oidcStateValue{ExpireAt: time.Now().Add(2 * time.Minute)})

	request := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=abc&state="+url.QueryEscape(state), nil)
	recorder := httptest.NewRecorder()

	OIDCCallbackHandler(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, recorder.Code)
	}

	location := recorder.Header().Get("Location")
	if !strings.Contains(location, "token=jwt-token") {
		t.Fatalf("expected jwt token in redirect, got %s", location)
	}
	if !strings.Contains(location, "username=authentik-user") {
		t.Fatalf("expected username in redirect, got %s", location)
	}
}
