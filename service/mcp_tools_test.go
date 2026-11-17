package service

import (
	"net/http"
	"testing"
)

func TestMCPAdaptersPreserveListShapes(t *testing.T) {
	old := mcpInvoke
	defer func() { mcpInvoke = old }()
	for _, tc := range []struct {
		name    string
		payload interface{}
		want    string
	}{
		{"list_country", map[string]interface{}{"items": []interface{}{" Canada ", "", 7}}, "Canada"},
		{"list_hashtags", map[string]interface{}{"items": []interface{}{map[string]interface{}{"tag": " coffee "}}}, "coffee"},
	} {
		mcpInvoke = func(*http.Request, http.HandlerFunc, string, map[string]interface{}, string, interface{}) (interface{}, error) {
			return tc.payload, nil
		}
		result, err := mcpExecuteTool(mcpTestRequest(`{}`), tc.name, map[string]interface{}{})
		if err != nil {
			t.Fatal(err)
		}
		items := result.([]string)
		if len(items) != 1 || items[0] != tc.want {
			t.Fatalf("%s: %v", tc.name, result)
		}
	}
}
func TestMCPAutoPaginationStopsOnEmptyPage(t *testing.T) {
	old := mcpInvoke
	defer func() { mcpInvoke = old }()
	calls := 0
	mcpInvoke = func(*http.Request, http.HandlerFunc, string, map[string]interface{}, string, interface{}) (interface{}, error) {
		calls++
		return map[string]interface{}{"items": []interface{}{}, "total": float64(10000)}, nil
	}
	result, err := mcpExecuteTool(mcpTestRequest(`{}`), "list_travel_plans", map[string]interface{}{})
	if err != nil || calls != 1 || result.(map[string]interface{})["total"] != 0 {
		t.Fatalf("%v %v calls=%d", result, err, calls)
	}
}
func TestMCPInvocationSanitizesInternalErrors(t *testing.T) {
	_, err := mcpInvokeIntegration(mcpTestRequest(`{}`), func(w http.ResponseWriter, r *http.Request) { http.Error(w, "SQL password=secret private-path", 500) }, "GET", nil, "", nil)
	if err == nil || err.Error() != "Internal Server Error" {
		t.Fatal(err)
	}
}

func TestMCPAllToolsDispatch(t *testing.T) {
	old := mcpInvoke
	defer func() { mcpInvoke = old }()
	for _, d := range mcpToolDefinitions {
		t.Run(d.Name, func(t *testing.T) {
			calls := 0
			mcpInvoke = func(_ *http.Request, _ http.HandlerFunc, method string, args map[string]interface{}, id string, body interface{}) (interface{}, error) {
				calls++
				if method == "GET" && id != "" {
					return map[string]interface{}{"id": float64(1), "daily_plans": []interface{}{map[string]interface{}{"id": float64(2), "summary": "old", "day_index": float64(1)}}}, nil
				}
				return map[string]interface{}{"items": []interface{}{}, "total": float64(0)}, nil
			}
			r := mcpTestRequest(`{}`)
			r = r.WithContext(ContextWithMCPPrincipal(r.Context(), &IntegrationPrincipal{Scopes: []string{"travel-plans:read", "travel-plans:write"}}))
			a := map[string]interface{}{"id": float64(1), "travel_plan_id": float64(1), "item_id": float64(2), "item": map[string]interface{}{"summary": "new"}, "radius": float64(100)}
			if _, err := mcpExecuteTool(r, d.Name, a); err != nil {
				t.Fatal(err)
			}
			if calls == 0 {
				t.Fatal("tool did not dispatch")
			}
		})
	}
}
