package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mcpTestRequest(body string) *http.Request {
	r := httptest.NewRequest("POST", "https://map.example/mcp", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r.WithContext(ContextWithMCPPrincipal(r.Context(), &IntegrationPrincipal{}))
}
func TestMCPProtocol(t *testing.T) {
	for _, tc := range []struct {
		body string
		want string
	}{
		{`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, `"protocolVersion":"2025-06-18"`},
		{`{"jsonrpc":"2.0","id":"x","method":"ping"}`, `"result":{}`},
		{`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, `"name":"delete_travel_plan_item"`},
		{`{"jsonrpc":"2.0","id":3,"method":"unknown"}`, `"code":-32601`},
		{`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_marker","arguments":{"id":-1}}}`, `invalid_input`},
		{`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"invented"}}`, `"code":-32602`},
		{`[]`, `"code":-32700`},
		{`{} {}`, `"code":-32700`},
	} {
		w := httptest.NewRecorder()
		MCPHandler(w, mcpTestRequest(tc.body))
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%s: %s", tc.body, w.Body.String())
		}
	}
}
func TestMCPRequiresIdentityAndRejectsMethods(t *testing.T) {
	w := httptest.NewRecorder()
	MCPHandler(w, httptest.NewRequest("POST", "/mcp", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatalf("anonymous status %d", w.Code)
	}
	r := mcpTestRequest(`{}`)
	r.Method = "GET"
	w = httptest.NewRecorder()
	MCPHandler(w, r)
	if w.Code != 405 {
		t.Fatalf("GET status %d", w.Code)
	}
	r = mcpTestRequest(`{}`)
	r.Header.Set("MCP-Protocol-Version", "bogus")
	w = httptest.NewRecorder()
	MCPHandler(w, r)
	if w.Code != 400 {
		t.Fatalf("version status %d", w.Code)
	}
}
func TestMCPNotificationsNeverExecuteTools(t *testing.T) {
	old := mcpInvoke
	defer func() { mcpInvoke = old }()
	called := false
	mcpInvoke = func(*http.Request, http.HandlerFunc, string, map[string]interface{}, string, interface{}) (interface{}, error) {
		called = true
		return nil, nil
	}
	w := httptest.NewRecorder()
	MCPHandler(w, mcpTestRequest(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"delete_travel_plan","arguments":{"id":1}}}`))
	if w.Code != 202 || called {
		t.Fatalf("notification executed=%v status=%d", called, w.Code)
	}
}
func TestMCPToolCompatibilityEnvelope(t *testing.T) {
	old := mcpInvoke
	defer func() { mcpInvoke = old }()
	mcpInvoke = func(r *http.Request, h http.HandlerFunc, method string, args map[string]interface{}, id string, body interface{}) (interface{}, error) {
		if method != "GET" || args["limit"] != 50 {
			t.Errorf("list args %v", args)
		}
		return map[string]interface{}{"items": []interface{}{}, "total": 0}, nil
	}
	w := httptest.NewRecorder()
	MCPHandler(w, mcpTestRequest(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_markers","arguments":{"limit":200}}}`))
	var envelope struct {
		Result struct{ Content []struct{ Text string } }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Result.Content) != 1 || !strings.Contains(envelope.Result.Content[0].Text, `"ok":true`) {
		t.Fatal(w.Body.String())
	}
}
func TestMCPInProcessInvocationPreservesContextAndDropsHeaders(t *testing.T) {
	r := mcpTestRequest(`{}`)
	r.Header.Set("X-API-Key", "attacker")
	r.Header.Set("x-roroad-api-base-url", "https://attacker.invalid")
	out, err := mcpInvokeIntegration(r, func(w http.ResponseWriter, q *http.Request) {
		if MCPPrincipalForContext(q.Context()) == nil || q.Header.Get("X-API-Key") != "" || q.Header.Get("x-roroad-api-base-url") != "" {
			t.Error("identity/header boundary violated")
		}
		respondJSON(w, 200, map[string]bool{"sameProcess": true})
	}, "GET", nil, "", nil)
	if err != nil || out.(map[string]interface{})["sameProcess"] != true {
		t.Fatalf("%v %v", out, err)
	}
}
