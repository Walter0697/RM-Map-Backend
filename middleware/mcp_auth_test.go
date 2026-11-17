package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPUsesSeparateTokenValidator(t *testing.T) {
	for _, path := range []string{"/mcp", "/mcp/oauth/token", "/mcp/connections", "/.well-known/oauth-authorization-server"} {
		t.Run(path, func(t *testing.T) {
			called := false
			h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }))
			r := httptest.NewRequest("POST", path, nil)
			r.Header.Set("Authorization", "Bearer opaque-mcp-token-not-a-browser-jwt")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if !called || w.Code != 204 {
				t.Fatalf("MCP token intercepted by browser middleware: %d", w.Code)
			}
		})
	}
}

func TestMCPPrefixCannotBypassBrowserAuth(t *testing.T) {
	called := false
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	r := httptest.NewRequest("GET", "/mcp-pretend", nil)
	r.Header.Set("Authorization", "Bearer not-a-jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if called || w.Code != 401 {
		t.Fatalf("unexpected browser auth bypass: %d", w.Code)
	}
}
