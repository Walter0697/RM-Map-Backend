package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

// mcpInvoke calls shared integration business handlers in-process. No network request,
// client-provided destination, persisted API key, or alternate authorization is used.
var mcpInvoke = mcpInvokeIntegration

func mcpInvokeIntegration(r *http.Request, h http.HandlerFunc, method string, args map[string]interface{}, id string, body interface{}) (interface{}, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req := r.Clone(r.Context())
	req.Method = method
	req.URL = &url.URL{Path: "/mcp/internal"}
	req.Body = io.NopCloser(bytes.NewReader(data))
	req.ContentLength = int64(len(data))
	req.Header = make(http.Header)
	req.Header.Set("Content-Type", "application/json")
	q := url.Values{}
	for key, v := range args {
		if a, ok := v.([]interface{}); ok {
			parts := []string{}
			for _, s := range a {
				if t, ok := s.(string); ok && strings.TrimSpace(t) != "" {
					parts = append(parts, strings.TrimSpace(t))
				}
			}
			q.Set(key, strings.Join(parts, ","))
		} else {
			q.Set(key, fmt.Sprint(v))
		}
	}
	req.URL.RawQuery = q.Encode()
	rc := chi.NewRouteContext()
	if id != "" {
		rc.URLParams.Add("id", id)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))
	recorder := &mcpResponse{header: make(http.Header)}
	h(recorder, req)
	if recorder.overflow {
		return nil, &mcpToolError{Code: "upstream_error", Message: "Result is too large; narrow the filters"}
	}
	if recorder.status >= 400 {
		code := "upstream_error"
		switch recorder.status {
		case 400, 422:
			code = "invalid_input"
		case 401:
			code = "upstream_auth_error"
		case 403:
			code = "upstream_auth_error"
		case 404:
			code = "upstream_not_found"
		case 429:
			code = "upstream_rate_limited"
		}
		// Do not expose SQL errors, internal paths, or provider credentials through tool errors.
		msg := http.StatusText(recorder.status)
		if recorder.status < 500 {
			var payload map[string]interface{}
			if json.Unmarshal(recorder.body.Bytes(), &payload) == nil {
				if s, ok := payload["message"].(string); ok {
					msg = s
				}
			}
		}
		return nil, &mcpToolError{Code: code, Message: msg, Status: recorder.status}
	}
	if recorder.body.Len() == 0 {
		return map[string]interface{}{}, nil
	}
	var result interface{}
	if err := json.Unmarshal(recorder.body.Bytes(), &result); err != nil {
		return nil, &mcpToolError{Code: "upstream_error", Message: "Invalid handler response"}
	}
	return result, nil
}

type mcpResponse struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (r *mcpResponse) Header() http.Header { return r.header }
func (r *mcpResponse) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}
func (r *mcpResponse) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = 200
	}
	if r.body.Len()+len(b) > 8*1024*1024 {
		r.overflow = true
		return len(b), nil
	}
	return r.body.Write(b)
}

func mcpExecuteTool(r *http.Request, name string, a map[string]interface{}) (interface{}, error) {
	get := func(h http.HandlerFunc, q map[string]interface{}, id string) (interface{}, error) {
		return mcpInvoke(r, h, http.MethodGet, q, id, nil)
	}
	id := func(key string) string { return strconv.FormatFloat(a[key].(float64), 'f', 0, 64) }
	list := func(h http.HandlerFunc) (interface{}, error) {
		a["limit"] = mcpListLimit(a["limit"])
		return get(h, a, "")
	}
	switch name {
	case "list_markers":
		return list(IntegrationListMarkersHandler)
	case "list_stations":
		return list(IntegrationListStationsHandler)
	case "list_schedules":
		return list(IntegrationListSchedulesHandler)
	case "list_markers_nearby", "search_nearby_markers":
		if _, ok := a["radius"]; !ok {
			a["radius"] = a["area"]
		}
		delete(a, "area")
		return list(IntegrationNearbySearchMarkersHandler)
	case "get_marker":
		return get(mcpGetMarkerHandler, nil, id("id"))
	case "get_schedule":
		return get(mcpGetScheduleHandler, nil, id("id"))
	case "list_country", "get_country_part", "list_hashtags":
		h := IntegrationListMarkerCountriesHandler
		if name == "get_country_part" {
			h = IntegrationListMarkerCountryPartsHandler
		}
		if name == "list_hashtags" {
			h = IntegrationListMarkerHashtagsHandler
		}
		result, err := get(h, a, "")
		if err != nil {
			return nil, err
		}
		values := []string{}
		page, _ := result.(map[string]interface{})
		items, _ := page["items"].([]interface{})
		for _, item := range items {
			if name == "list_hashtags" {
				if m, ok := item.(map[string]interface{}); ok {
					item = m["tag"]
				}
			}
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				values = append(values, strings.TrimSpace(s))
			}
		}
		return values, nil
	case "list_marker_types", "list_travel_plans":
		size := 50
		h := IntegrationListTravelPlansHandler
		if name == "list_marker_types" {
			size = 200
			h = IntegrationListSettingsMarkerTypesHandler
		}
		items := []interface{}{}
		values := []string{}
		// Bounded auto-pagination prevents an inconsistent upstream total from looping forever.
		for offset := 0; offset < 10000; {
			a["limit"] = size
			a["offset"] = offset
			result, err := get(h, a, "")
			if err != nil {
				return nil, err
			}
			page, _ := result.(map[string]interface{})
			batch, _ := page["items"].([]interface{})
			if len(batch) == 0 {
				break
			}
			if name == "list_marker_types" {
				for _, item := range batch {
					m, ok := item.(map[string]interface{})
					if !ok || m["hidden"] == true {
						continue
					}
					if s, ok := m["value"].(string); ok && strings.TrimSpace(s) != "" {
						values = append(values, strings.TrimSpace(s))
					}
				}
			} else {
				items = append(items, batch...)
			}
			offset += len(batch)
			if total, ok := page["total"].(float64); ok && float64(offset) >= total {
				break
			}
			if len(batch) < size {
				break
			}
			if offset >= 10000 {
				return nil, &mcpToolError{Code: "invalid_input", Message: "Too many results; narrow the filters"}
			}
		}
		if name == "list_marker_types" {
			return values, nil
		}
		return map[string]interface{}{"items": items, "total": len(items)}, nil
	case "get_travel_plan":
		return get(IntegrationGetTravelPlanHandler, nil, id("id"))
	case "create_travel_plan":
		if _, ok := a["daily_plans"]; !ok {
			a["daily_plans"] = []interface{}{}
		}
		return mcpInvoke(r, IntegrationCreateTravelPlanHandler, http.MethodPost, nil, "", a)
	case "update_travel_plan", "update_travel_plan_status":
		planID := id("id")
		delete(a, "id")
		return mcpInvoke(r, IntegrationUpdateTravelPlanHandler, http.MethodPut, nil, planID, a)
	case "delete_travel_plan":
		return mcpInvoke(r, IntegrationDeleteTravelPlanHandler, http.MethodDelete, nil, id("id"), nil)
	case "overwrite_schedules_by_date":
		return mcpInvoke(r, IntegrationOverwriteSchedulesByDateHandler, http.MethodPut, nil, "", a)
	case "preview_geocode":
		return mcpInvoke(r, IntegrationGeocodeStaticMapPreviewHandler, http.MethodPost, nil, "", a)
	case "create_travel_plan_item", "update_travel_plan_item", "delete_travel_plan_item":
		return mcpTravelPlanItem(r, name, a)
	}
	return nil, mcpInvalid("tool")
}
func mcpListLimit(v interface{}) int {
	if n, ok := v.(float64); ok {
		if n > 50 {
			return 50
		}
		if n < 1 {
			return 1
		}
		return int(n)
	}
	return 20
}

// Keep the legacy item-tool result shape (full plan detail). The compatibility
// operation replaces daily_plans, just as the original MCP adapter did.
func mcpTravelPlanItem(r *http.Request, name string, a map[string]interface{}) (interface{}, error) {
	p := MCPPrincipalForContext(r.Context())
	if p == nil || !p.HasScope(constant.APIKeyScopeTravelPlansWrite) || !p.HasScope(constant.APIKeyScopeTravelPlansRead) {
		return nil, &mcpToolError{Code: "upstream_auth_error", Message: "Travel plan read and write scopes required", Status: 403}
	}
	planID := strconv.FormatFloat(a["travel_plan_id"].(float64), 'f', 0, 64)
	result, err := mcpInvoke(r, IntegrationGetTravelPlanHandler, http.MethodGet, nil, planID, nil)
	if err != nil {
		return nil, err
	}
	detail, ok := result.(map[string]interface{})
	if !ok {
		return nil, &mcpToolError{Code: "upstream_error", Message: "Invalid travel plan detail"}
	}
	items, _ := detail["daily_plans"].([]interface{})
	next := []interface{}{}
	found := false
	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			return nil, &mcpToolError{Code: "upstream_error", Message: "Invalid travel plan entry"}
		}
		if name != "create_travel_plan_item" && item["id"] == a["item_id"] {
			found = true
			if name == "delete_travel_plan_item" {
				continue
			}
			for k, v := range a["item"].(map[string]interface{}) {
				item[k] = v
			}
		}
		writable := map[string]interface{}{}
		for _, k := range []string{"schedule_id", "day_index", "local_date", "summary", "details"} {
			if v, ok := item[k]; ok && v != nil {
				writable[k] = v
			}
		}
		next = append(next, writable)
	}
	if name == "create_travel_plan_item" {
		next = append(next, a["item"])
	} else if !found {
		return nil, &mcpToolError{Code: "upstream_not_found", Message: "Travel plan item not found", Status: 404}
	}
	return mcpInvoke(r, IntegrationUpdateTravelPlanHandler, http.MethodPut, nil, planID, map[string]interface{}{"daily_plans": next})
}

func mcpGetMarkerHandler(w http.ResponseWriter, r *http.Request) {
	p, ok := authenticateIntegrationPrincipal(w, r, "integration.markers.get", constant.APIKeyScopeMarkersRead, "")
	if !ok {
		return
	}
	var marker dbmodel.Marker
	query := database.Connection.Preload("RestaurantInfo").Where("id = ? AND relation_id = ?", chi.URLParam(r, "id"), p.Relation.ID).Where(`type IN (SELECT value FROM marker_types WHERE hidden IS DISTINCT FROM TRUE)`)
	if !canAccessTestingEntities(p.ActorRole()) {
		query = query.Where("testing = ?", false)
	}
	if err := query.First(&marker).Error; err != nil {
		mcpDetailError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, buildIntegrationMarkerResponse(helper.ConvertMarker(marker), marker.Testing))
}
func mcpGetScheduleHandler(w http.ResponseWriter, r *http.Request) {
	p, ok := authenticateIntegrationPrincipal(w, r, "integration.schedules.get", constant.APIKeyScopeSchedulesRead, "")
	if !ok {
		return
	}
	var schedule dbmodel.Schedule
	query := database.Connection.Preload("SelectedMarker.RestaurantInfo").Where("id = ? AND relation_id = ?", chi.URLParam(r, "id"), p.Relation.ID)
	if !canAccessTestingEntities(p.ActorRole()) {
		query = query.Where("testing = ?", false)
	}
	if err := query.First(&schedule).Error; err != nil {
		mcpDetailError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, buildIntegrationScheduleResponse(schedule, nil, nil))
}
func mcpDetailError(w http.ResponseWriter, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "not found", 404)
	} else {
		http.Error(w, "unable to read record", 500)
	}
}
