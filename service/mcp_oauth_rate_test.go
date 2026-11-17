package service

import (
	"net/http/httptest"
	"testing"
)

func TestMCPRateLimitIgnoresSpoofedForwardedAddress(t *testing.T) {
	r := httptest.NewRequest("POST", "/mcp/oauth/register", nil)
	r.RemoteAddr = "192.0.2.29:4444"
	operation := "test-" + mcpRandom()
	if !mcpRateLimit(httptest.NewRecorder(), r, operation, 1) {
		t.Fatal("first request denied")
	}
	r.Header.Set("X-Forwarded-For", "192.0.2.77")
	w := httptest.NewRecorder()
	if mcpRateLimit(w, r, operation, 1) || w.Code != 429 {
		t.Fatal("forwarded address bypassed limit")
	}
}
