package service

import (
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBuildAndParseRawAPIKey(t *testing.T) {
	raw := BuildRawAPIKey(42, "secret-value")

	id, secret, err := ParseRawAPIKey(raw)
	if err != nil {
		t.Fatalf("ParseRawAPIKey returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id 42, got %d", id)
	}
	if secret != "secret-value" {
		t.Fatalf("unexpected secret parsed: %s", secret)
	}
}

func TestParseRawAPIKeyInvalidValues(t *testing.T) {
	invalid := []string{
		"",
		"abc",
		"rmk_bad",
		"rmk_0_secret",
		"rmk_1_",
	}

	for _, value := range invalid {
		if _, _, err := ParseRawAPIKey(value); err == nil {
			t.Fatalf("expected parse error for %q", value)
		}
	}
}

func TestHasScope(t *testing.T) {
	key := &dbmodel.APIKey{Scopes: "markers:read,schedules:write,static-preview:generate"}
	if !HasScope(key, "markers:read") {
		t.Fatalf("expected markers:read scope")
	}
	if !HasScope(key, "static-preview:generate") {
		t.Fatalf("expected static-preview:generate scope")
	}
	if HasScope(key, "markers:write") {
		t.Fatalf("did not expect markers:write scope")
	}
	if HasScope(key, "settings:read") {
		t.Fatalf("did not expect settings:read scope")
	}
}

func TestRequestSourceIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:3456"

	if got := requestSourceIP(req); got != "10.0.0.2" {
		t.Fatalf("expected remote ip 10.0.0.2, got %s", got)
	}

	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.2")
	if got := requestSourceIP(req); got != "203.0.113.9" {
		t.Fatalf("expected forwarded ip 203.0.113.9, got %s", got)
	}
}

func TestNormalizeScopesFiltersInvalidValues(t *testing.T) {
	result := normalizeScopes([]string{
		"markers:read",
		"markers:read",
		"static-preview:generate",
		"invalid:scope",
		" settings:write ",
		"",
	})
	if len(result) != 3 {
		t.Fatalf("expected 3 scopes, got %d (%v)", len(result), result)
	}
	if result[0] != "markers:read" || result[1] != "settings:write" || result[2] != "static-preview:generate" {
		t.Fatalf("unexpected normalized scopes: %v", result)
	}
}

func TestParseIntegrationListQuery(t *testing.T) {
	values := url.Values{}
	values.Set("limit", "25")
	values.Set("offset", "10")
	values.Set("cursor", "123")
	values.Set("sort_by", "label")
	values.Set("order", "desc")
	values.Set("status", "active")

	_, err := parseIntegrationListQuery(values, map[string]string{
		"label": "label",
	}, "label", []string{"status"})
	if err == nil {
		t.Fatalf("expected parseIntegrationListQuery to reject cursor with offset")
	}
}

func TestParseIntegrationListQueryCursor(t *testing.T) {
	values := url.Values{}
	values.Set("limit", "25")
	values.Set("cursor", "123")
	values.Set("sort_by", "label")
	values.Set("order", "desc")
	values.Set("status", "active")

	query, err := parseIntegrationListQuery(values, map[string]string{
		"label": "label",
	}, "label", []string{"status"})
	if err != nil {
		t.Fatalf("parseIntegrationListQuery returned error: %v", err)
	}

	if query.Limit != 25 || query.Cursor != 123 || query.SortBy != "label" || query.Order != "desc" {
		t.Fatalf("unexpected parsed query: %+v", query)
	}
	if query.Filters["status"] != "active" {
		t.Fatalf("expected status filter to be active, got %q", query.Filters["status"])
	}
}

func TestParseIntegrationListQueryInvalidCursor(t *testing.T) {
	values := url.Values{}
	values.Set("cursor", "abc")

	_, err := parseIntegrationListQuery(values, map[string]string{"label": "label"}, "label", []string{"status"})
	if err == nil {
		t.Fatalf("expected error for invalid cursor")
	}
}

func TestParseIntegrationListQueryRejectsUnsupportedFilter(t *testing.T) {
	values := url.Values{}
	values.Set("unknown", "x")

	_, err := parseIntegrationListQuery(values, map[string]string{"label": "label"}, "label", []string{"status"})
	if err == nil {
		t.Fatalf("expected error for unsupported filter")
	}
}

func TestParseIntegrationListQuerySupportsMultiSearchValues(t *testing.T) {
	values := url.Values{}
	values.Add("search", "food")
	values.Add("search", "museum")

	query, err := parseIntegrationListQuery(values, map[string]string{"label": "label"}, "label", []string{"search"})
	if err != nil {
		t.Fatalf("parseIntegrationListQuery returned error: %v", err)
	}
	if query.Filters["search"] != "food,museum" {
		t.Fatalf("expected combined search terms, got %q", query.Filters["search"])
	}
}

func TestParseIntegrationPageQuery(t *testing.T) {
	values := url.Values{}
	values.Set("page", "3")
	values.Set("page_size", "25")
	values.Set("sort_by", "label")
	values.Set("order", "desc")
	values.Set("status", "active")

	query, err := parseIntegrationPageQuery(values, map[string]string{
		"label": "label",
	}, "label", []string{"status"})
	if err != nil {
		t.Fatalf("parseIntegrationPageQuery returned error: %v", err)
	}

	if query.Page != 3 || query.PageSize != 25 || query.SortBy != "label" || query.Order != "desc" {
		t.Fatalf("unexpected parsed page query: %+v", query)
	}
	if query.Filters["status"] != "active" {
		t.Fatalf("expected status filter to be active, got %q", query.Filters["status"])
	}
}

func TestParseIntegrationPageQueryInvalidPage(t *testing.T) {
	values := url.Values{}
	values.Set("page", "0")

	_, err := parseIntegrationPageQuery(values, map[string]string{"label": "label"}, "label", []string{"status"})
	if err == nil {
		t.Fatalf("expected error for invalid page")
	}
}

func TestParseIntegrationPageQueryRejectsUnsupportedFilter(t *testing.T) {
	values := url.Values{}
	values.Set("unknown", "x")

	_, err := parseIntegrationPageQuery(values, map[string]string{"label": "label"}, "label", []string{"status"})
	if err == nil {
		t.Fatalf("expected error for unsupported filter")
	}
}

func TestIntegrationPageResponse(t *testing.T) {
	payload := integrationPageResponse([]string{"a", "b"}, 23, integrationPageQuery{
		Page:     2,
		PageSize: 10,
		SortBy:   "updated_at",
		Order:    "desc",
	})

	if payload["page"] != 2 {
		t.Fatalf("expected page 2, got %+v", payload["page"])
	}
	if payload["page_size"] != 10 {
		t.Fatalf("expected page_size 10, got %+v", payload["page_size"])
	}
	if payload["total_pages"] != 3 {
		t.Fatalf("expected total_pages 3, got %+v", payload["total_pages"])
	}
	if payload["has_next"] != true {
		t.Fatalf("expected has_next true, got %+v", payload["has_next"])
	}
	if payload["has_prev"] != true {
		t.Fatalf("expected has_prev true, got %+v", payload["has_prev"])
	}
}

func TestNormalizeSettingsPinLabelFallsBackToValue(t *testing.T) {
	value := "canonical-value"
	if got := normalizeSettingsPinLabel(value, nil); got != value {
		t.Fatalf("expected fallback to value for nil label, got %q", got)
	}

	empty := ""
	if got := normalizeSettingsPinLabel(value, &empty); got != value {
		t.Fatalf("expected fallback to value for empty label, got %q", got)
	}

	whitespace := "   \t  "
	if got := normalizeSettingsPinLabel(value, &whitespace); got != value {
		t.Fatalf("expected fallback to value for whitespace label, got %q", got)
	}
}

func TestNormalizeSettingsPinLabelUsesTrimmedLabel(t *testing.T) {
	value := "canonical-value"
	label := "  Friendly Label  "

	if got := normalizeSettingsPinLabel(value, &label); got != "Friendly Label" {
		t.Fatalf("expected trimmed settings label, got %q", got)
	}
}

func TestToIntegrationSettingsPinResponse(t *testing.T) {
	settingsLabel := "  Scenic Pin  "
	pin := dbmodel.Pin{
		ObjectBase: dbmodel.ObjectBase{
			BaseModel: dbmodel.BaseModel{ID: 77},
		},
		Label:         "pin-value",
		SettingsLabel: &settingsLabel,
		ImagePath:     "/pins/original.png",
		DisplayPath:   "/pins/display.png",
		TopLeftX:      10,
		TopLeftY:      20,
		BottomRightX:  30,
		BottomRightY:  40,
	}

	response := toIntegrationSettingsPinResponse(pin)
	if response.ID != 77 {
		t.Fatalf("expected id=77, got %d", response.ID)
	}
	if response.Value != "pin-value" {
		t.Fatalf("expected value pin-value, got %q", response.Value)
	}
	if response.Label != "Scenic Pin" {
		t.Fatalf("expected normalized label Scenic Pin, got %q", response.Label)
	}
	if response.ImagePath != "/pins/original.png" || response.DisplayPath != "/pins/display.png" {
		t.Fatalf("unexpected image fields in response: %+v", response)
	}
	if response.TopLeftX != 10 || response.TopLeftY != 20 || response.BottomRightX != 30 || response.BottomRightY != 40 {
		t.Fatalf("unexpected bounds in response: %+v", response)
	}
}
