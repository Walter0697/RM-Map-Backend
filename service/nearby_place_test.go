package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mapmarker/backend/constant"
	"mapmarker/backend/database/dbmodel"
)

func TestNearbySearchQueryRadiusCompatibility(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  float64
	}{
		{"radius_meters=3000", 3000},
		{"radius=3", 3},
		{"area=200", 200},
		{"radius_meters=3000&radius=3&area=2", 3000},
		{"radius=200&area=100", 200},
	} {
		q, err := parseIntegrationNearbySearchQuery(httptest.NewRequest(http.MethodGet, "/?latitude=0&longitude=0&"+tc.query, nil))
		if err != nil || q.RadiusMeters != tc.want {
			t.Errorf("%s: got %+v, %v; want %v meters", tc.query, q, err, tc.want)
		}
	}
	for _, query := range []string{
		"radius_meters=3000",
		"latitude=NaN&longitude=0&radius=2",
		"latitude=0&longitude=NaN&radius=2",
		"latitude=0&longitude=0&radius=NaN",
		"latitude=0&longitude=0&radius_meters=50001",
		"latitude=0&longitude=0&place=test&radius_meters=3000",
		"place=test&place_country_code=CAN&radius_meters=3000",
	} {
		if _, err := parseIntegrationNearbySearchQuery(httptest.NewRequest(http.MethodGet, "/?"+query, nil)); err == nil {
			t.Errorf("expected rejection for %s", query)
		}
	}
}

func TestNearbySearchMCPNamedPlace(t *testing.T) {
	resetIntegrationMarkerHooks()
	t.Cleanup(resetIntegrationMarkerHooks)
	t.Cleanup(func() { integrationResolveNearbyPlaceFn = resolveNearbyPlace })

	first := nearbyPlaceCandidate{Label: "Don Mills Station", Address: "Toronto", Latitude: 43.775, Longitude: -79.347}
	other := nearbyPlaceCandidate{Label: "Alternate", Latitude: 43, Longitude: -79}
	for _, tc := range []struct {
		name      string
		geocode   bool
		places    []nearbyPlaceCandidate
		lookupErr error
		status    string
		httpError int
		search    bool
		mcpPlace  bool
	}{
		{name: "unique empty nearby results", geocode: true, places: []nearbyPlaceCandidate{first}, status: "ok", search: true},
		{name: "ambiguous", geocode: true, places: []nearbyPlaceCandidate{first, other}, status: "place_ambiguous"},
		{name: "not found", geocode: true, places: []nearbyPlaceCandidate{}, status: "place_not_found"},
		{name: "MCP marker read permits place lookup", places: []nearbyPlaceCandidate{first}, status: "ok", search: true, mcpPlace: true},
		{name: "provider error", geocode: true, lookupErr: errors.New("private provider error"), httpError: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved, searched := false, false
			integrationResolveNearbyPlaceFn = func(_ context.Context, place, country string) ([]nearbyPlaceCandidate, error) {
				resolved = true
				if place != "Don Mills Station, Toronto" || country != "CA" {
					t.Fatalf("wrong provider arguments: %q %q", place, country)
				}
				return tc.places, tc.lookupErr
			}
			integrationFindNearbyMarkersFn = func(relation dbmodel.UserRelation, q integrationNearbySearchQuery, testing bool) ([]integrationNearbyMarkerRow, error) {
				searched = true
				if relation.ID != 7 || testing || q.Latitude != first.Latitude || q.Longitude != first.Longitude || q.MarkerType != "attraction" || q.ExcludeMarkerType != "food" || q.RadiusMeters != 3000 || q.Limit != 10 {
					t.Fatalf("wrong scoped search: relation=%d testing=%t query=%+v", relation.ID, testing, q)
				}
				return nil, nil
			}
			user := dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 9}, IsActivated: true}
			relation := dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 7}, UserOneUID: 9}
			scopes := []string{constant.APIKeyScopeMarkersRead}
			if tc.geocode {
				scopes = append(scopes, constant.APIKeyScopeStaticPreview)
			}
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req = req.WithContext(ContextWithMCPPrincipal(req.Context(), NewOAuthIntegrationPrincipal(user, relation, scopes)))
			result, err := mcpExecuteTool(req, "list_markers_nearby", map[string]interface{}{
				"place": "Don Mills Station, Toronto", "place_country_code": "ca", "radius_meters": float64(3000),
				"type": "attraction", "exclude_type": "food", "limit": float64(10),
			})
			if searched != tc.search || resolved != (tc.geocode || tc.mcpPlace) {
				t.Fatalf("unexpected calls: searched=%t resolved=%t", searched, resolved)
			}
			if tc.httpError != 0 {
				toolErr, ok := err.(*mcpToolError)
				if !ok || toolErr.Status != tc.httpError || toolErr.Message == "private provider error" {
					t.Fatalf("wrong error: %+v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			page := result.(map[string]interface{})
			items, ok := page["items"].([]interface{})
			if !ok || len(items) != 0 || page["status"] != tc.status || page["radius_meters"] != float64(3000) {
				t.Fatalf("wrong response: %+v", page)
			}
			if tc.search {
				origin := page["origin"].(map[string]interface{})
				filters := page["filters"].(map[string]interface{})
				if origin["label"] != first.Label || origin["latitude"] != first.Latitude || filters["exclude_type"] != "food" {
					t.Fatalf("missing effective query metadata: %+v", page)
				}
			} else if _, hasOrigin := page["origin"]; hasOrigin || len(page["candidates"].([]interface{})) != len(tc.places) {
				t.Fatalf("wrong unresolved response: %+v", page)
			}
		})
	}
}

func TestNearbyPlaceProviderDecoding(t *testing.T) {
	raw := json.RawMessage(`{"results":[
		{"poi":{"name":"Don Mills Station"},"address":{"freeformAddress":"Toronto"},"position":{"lat":43.775,"lon":-79.347}},
		{"poi":{"name":"Don Mills Station"},"address":{"freeformAddress":"Toronto"},"position":{"lat":43.775,"lon":-79.347}},
		{"poi":{"name":"Missing longitude"},"position":{"lat":43}},
		{"address":{"freeformAddress":"Null Island"},"position":{"lat":0,"lon":0}},
		{"poi":{"name":"Bad coordinates"},"position":{"lat":181,"lon":0}}
	]}`)
	places, err := decodeNearbyPlaceCandidates(raw)
	if err != nil || len(places) != 2 || places[0].Label != "Don Mills Station" || places[1].Label != "Null Island" {
		t.Fatalf("wrong candidates: %+v (%v)", places, err)
	}
	if _, err := decodeNearbyPlaceCandidates([]byte(`{"error":"bad payload"}`)); err == nil {
		t.Fatal("missing results must not look like a successful empty lookup")
	}
}
