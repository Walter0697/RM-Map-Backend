package service

import (
	"net/http"
	"strings"
	"time"
)

var apiUsageRequireAdminFn = requireAdmin

func AdminExternalAPIUsageProvidersHandler(w http.ResponseWriter, r *http.Request) {
	if apiUsageRequireAdminFn(w, r) == nil {
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items": listManagedExternalAPIProvidersFn(),
	})
}

func AdminExternalAPIUsageSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if apiUsageRequireAdminFn(w, r) == nil {
		return
	}

	filter, err := parseExternalAPIUsageFilter(map[string]string{
		"from":     r.URL.Query().Get("from"),
		"to":       r.URL.Query().Get("to"),
		"provider": r.URL.Query().Get("provider"),
	}, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := listExternalAPIUsageSummaryFn(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.TrimSpace(result.Provider) == "" {
		result.Provider = "all"
	}
	respondJSON(w, http.StatusOK, result)
}

func AdminExternalAPIUsageTrendsHandler(w http.ResponseWriter, r *http.Request) {
	if apiUsageRequireAdminFn(w, r) == nil {
		return
	}

	filter, err := parseExternalAPIUsageFilter(map[string]string{
		"from":     r.URL.Query().Get("from"),
		"to":       r.URL.Query().Get("to"),
		"provider": r.URL.Query().Get("provider"),
	}, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	interval := strings.TrimSpace(r.URL.Query().Get("interval"))
	result, err := listExternalAPIUsageTrendsFn(filter, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(result.Provider) == "" {
		result.Provider = "all"
	}
	respondJSON(w, http.StatusOK, result)
}
