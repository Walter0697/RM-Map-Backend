package service

import (
	"mapmarker/backend/config"
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

func TestMCPMarkerFrontendOrigin(t *testing.T) {
	for _, tc := range []struct{ login, telegram, allowed, want string }{
		{"https://maps.example/login?next=x", "https://notify.example", "https://fallback.example", "https://maps.example"},
		{"javascript:bad", "https://notify.example/", "*", "https://notify.example"},
		{"", "", "http://localhost:3000/", "http://localhost:3000"},
		{"https://username:password@maps.example/login", "", "https://one.example,https://two.example", ""},
		{"", "", "https://*.example.com", ""},
		{"", "", "https://api.example/somepath", ""},
	} {
		if got := mcpSelectMarkerFrontendOrigin(tc.login, tc.telegram, tc.allowed); got != tc.want {
			t.Errorf("origin: got %q, want %q", got, tc.want)
		}
	}
}

func TestMCPMarkerToolsReturnPrimaryMapLink(t *testing.T) {
	oldInvoke, oldConfig := mcpInvoke, config.Data
	defer func() { mcpInvoke, config.Data = oldInvoke, oldConfig }()
	config.Data.OIDC.FrontendRedirectURL = "https://maps.example/login"
	for _, tool := range []string{"list_markers", "list_markers_nearby", "search_nearby_markers", "get_marker"} {
		t.Run(tool, func(t *testing.T) {
			marker := map[string]interface{}{
				"id": float64(12), "label": "Coffee shop", "distance_meters": float64(123),
				"link": "https://cafe.example", "social_media_link": "https://www.instagram.com/p/post/",
			}
			mcpInvoke = func(*http.Request, http.HandlerFunc, string, map[string]interface{}, string, interface{}) (interface{}, error) {
				if tool == "get_marker" {
					return marker, nil
				}
				return map[string]interface{}{"items": []interface{}{marker}, "status": "ok", "total": float64(1)}, nil
			}
			payload, err := mcpExecuteTool(mcpTestRequest(`{}`), tool, map[string]interface{}{"id": float64(12)})
			if err != nil {
				t.Fatal(err)
			}
			result := payload.(map[string]interface{})
			if tool != "get_marker" {
				if result["status"] != "ok" || result["total"] != float64(1) {
					t.Fatalf("changed pagination or metadata: %v", result)
				}
				result = result["items"].([]interface{})[0].(map[string]interface{})
			}
			if result["roroadmap_url"] != "https://maps.example/markers/12" || result["source_url"] != result["social_media_link"] ||
				result["link"] != "https://cafe.example" || result["distance_meters"] != float64(123) {
				t.Fatalf("marker link or legacy fields mismatch: %v", result)
			}
		})
	}
}

func TestMCPMarkerLinksOmitInvalidOrUnavailableLinks(t *testing.T) {
	marker := map[string]interface{}{"id": float64(9), "social_media_link": "javascript:bad", "link": "https://cafe.example"}
	mcpAddMarkerLinks(marker, "")
	if _, ok := marker["roroadmap_url"]; ok {
		t.Fatal("generated a map link without a configured origin")
	}
	if marker["source_url"] != "https://cafe.example" {
		t.Fatalf("expected external fallback: %v", marker)
	}

	marker = map[string]interface{}{"id": float64(1.5), "link": "https://user:pass@cafe.example"}
	mcpAddMarkerLinks(marker, "https://maps.example")
	for _, key := range []string{"roroadmap_url", "source_url"} {
		if _, ok := marker[key]; ok {
			t.Fatalf("generated %q from invalid data: %v", key, marker)
		}
	}
}
