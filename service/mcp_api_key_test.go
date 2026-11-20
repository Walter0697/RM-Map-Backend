package service

import (
	"mapmarker/backend/constant"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPAuthenticateUsesAPIKeyWhenExplicitlyEnabled(t *testing.T) {
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_ORIGIN", "https://rm.example")
	t.Setenv("MCP_API_KEY_ENABLED", "true")

	original := mcpAPIKeyPrincipalFn
	t.Cleanup(func() { mcpAPIKeyPrincipalFn = original })
	called := false
	mcpAPIKeyPrincipalFn = func(w http.ResponseWriter, r *http.Request) (*IntegrationPrincipal, bool) {
		called = true
		if got := r.Header.Get("X-API-Key"); got != "rmk_test" {
			t.Fatalf("API key header = %q", got)
		}
		return &IntegrationPrincipal{Scopes: []string{constant.APIKeyScopeMCPAccess}}, true
	}

	handlerCalled := false
	h := MCPAuthenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		if MCPPrincipalForContext(r.Context()) == nil {
			t.Fatal("MCP principal was not attached to request context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	r := httptest.NewRequest("POST", "https://rm.example/mcp", nil)
	r.Header.Set("X-API-Key", "rmk_test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if !called || !handlerCalled || w.Code != http.StatusNoContent {
		t.Fatalf("API-key MCP request was not accepted: called=%v handler=%v status=%d body=%s", called, handlerCalled, w.Code, w.Body.String())
	}
}

func TestMCPAuthenticateKeepsOAuthPathWhenAPIKeySupportIsDisabled(t *testing.T) {
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_ORIGIN", "https://rm.example")
	t.Setenv("MCP_API_KEY_ENABLED", "false")

	original := mcpAPIKeyPrincipalFn
	t.Cleanup(func() { mcpAPIKeyPrincipalFn = original })
	called := false
	mcpAPIKeyPrincipalFn = func(w http.ResponseWriter, r *http.Request) (*IntegrationPrincipal, bool) {
		called = true
		return nil, false
	}

	handlerCalled := false
	h := MCPAuthenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handlerCalled = true }))
	r := httptest.NewRequest("POST", "https://rm.example/mcp", nil)
	r.Header.Set("X-API-Key", "rmk_test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if called || handlerCalled || w.Code != http.StatusUnauthorized {
		t.Fatalf("disabled API-key support changed OAuth behavior: auth=%v handler=%v status=%d", called, handlerCalled, w.Code)
	}
}

func TestMCPAuthenticateDoesNotTreatBearerAsAPIKey(t *testing.T) {
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_ORIGIN", "https://rm.example")
	t.Setenv("MCP_API_KEY_ENABLED", "true")

	original := mcpAPIKeyPrincipalFn
	t.Cleanup(func() { mcpAPIKeyPrincipalFn = original })
	called := false
	mcpAPIKeyPrincipalFn = func(w http.ResponseWriter, r *http.Request) (*IntegrationPrincipal, bool) {
		called = true
		return nil, false
	}

	r := httptest.NewRequest("POST", "https://rm.example/mcp", nil)
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 257))
	w := httptest.NewRecorder()
	MCPAuthenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(w, r)

	if called || w.Code != http.StatusUnauthorized || !strings.Contains(w.Header().Get("WWW-Authenticate"), "resource_metadata") {
		t.Fatalf("Bearer credential was routed to API-key auth: called=%v status=%d", called, w.Code)
	}
}

func TestMCPAPIKeyCredentialPresentRecognizesOnlySupportedHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		value  string
		want   bool
	}{
		{name: "x api key", header: "X-API-Key", value: "rmk_test", want: true},
		{name: "api key authorization", header: "Authorization", value: "ApiKey rmk_test", want: true},
		{name: "bearer authorization", header: "Authorization", value: "Bearer rmk_test", want: false},
		{name: "query only", header: "", value: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "https://rm.example/mcp?api_key=rmk_test", nil)
			if tc.header != "" {
				r.Header.Set(tc.header, tc.value)
			}
			if got := mcpAPIKeyCredentialPresent(r); got != tc.want {
				t.Fatalf("credential presence = %v, want %v", got, tc.want)
			}
		})
	}
}
