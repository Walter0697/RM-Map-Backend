package service

import (
	"encoding/json"
	"testing"
)

func TestMCPContracts(t *testing.T) {
	names := []string{"list_markers", "list_country", "get_country_part", "list_hashtags", "list_markers_nearby", "get_marker", "list_marker_types", "list_stations", "list_schedules", "get_schedule", "overwrite_schedules_by_date", "list_travel_plans", "get_travel_plan", "create_travel_plan", "update_travel_plan", "update_travel_plan_status", "delete_travel_plan", "create_travel_plan_item", "update_travel_plan_item", "delete_travel_plan_item", "search_nearby_markers", "preview_geocode"}
	if len(names) != len(mcpToolDefinitions) {
		t.Fatalf("tool count %d", len(mcpToolDefinitions))
	}
	for i, n := range names {
		if mcpToolDefinitions[i].Name != n {
			t.Errorf("contract %d changed", i)
		}
	}
}
func TestMCPSchemaValidation(t *testing.T) {
	cases := []struct {
		name, args string
		valid      bool
	}{
		{"list_markers", `{"limit":200,"search":["coffee","tea"]}`, true},
		{"list_markers", `{"limit":201}`, false}, {"list_markers", `{"search":[]}`, false},
		{"get_marker", `{"id":1.5}`, false}, {"get_marker", `{"id":1}`, true},
		{"list_country", `{"unexpected":1}`, false},
		{"search_nearby_markers", `{"latitude":43,"longitude":-79}`, false},
		{"search_nearby_markers", `{"latitude":43,"longitude":-79,"area":100}`, true},
		{"list_markers_nearby", `{"place":"Don Mills Station, Toronto","place_country_code":"CA","radius_meters":3000,"exclude_type":"food"}`, true},
		{"list_markers_nearby", `{"latitude":43.775,"longitude":-79.347,"radius_meters":3000}`, true},
		{"list_markers_nearby", `{"latitude":43.775,"longitude":-79.347,"radius":3000}`, true},
		{"list_markers_nearby", `{"place":"Don Mills Station, Toronto"}`, false},
		{"list_markers_nearby", `{"latitude":43.775,"radius_meters":3000}`, false},
		{"search_nearby_markers", `{"place":"Don Mills Station, Toronto","radius_meters":3000}`, false},
		{"list_markers_nearby", `{"latitude":91,"longitude":0,"radius":100}`, false},
		{"update_travel_plan_item", `{"travel_plan_id":1,"item_id":2,"item":{}}`, false},
		{"update_travel_plan_item", `{"travel_plan_id":1,"item_id":2,"item":{"unknown":true}}`, false},
		{"overwrite_schedules_by_date", `{"date":"2026-11-17","items":[{"label":"visit","marker_id":1,"selected_time":"10:00"}]}`, true},
		{"create_travel_plan", `{"username":"user","title":"Trip","daily_plans":[{"summary":"Visit","day_index":0}]}`, false},
		{"preview_geocode", `{"street_number":"1","street_name":"Main","country":"Canada"}`, true},
	}
	for _, tc := range cases {
		var args interface{}
		json.Unmarshal([]byte(tc.args), &args)
		for _, d := range mcpToolDefinitions {
			if d.Name == tc.name {
				_, err := mcpValidate(d.InputSchema, args, "args")
				if (err == nil) != tc.valid {
					t.Errorf("%s %s: %v", tc.name, tc.args, err)
				}
			}
		}
	}
}
