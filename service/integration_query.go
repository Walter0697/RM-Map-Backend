package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultIntegrationLimit = 50
	maxIntegrationLimit     = 200
)

type integrationListQuery struct {
	Limit   int
	Offset  int
	SortBy  string
	Order   string
	Filters map[string]string
}

func parseIntegrationListQuery(values url.Values, allowedSort map[string]string, defaultSort string, allowedFilters []string) (integrationListQuery, error) {
	query := integrationListQuery{
		Limit:   defaultIntegrationLimit,
		Offset:  0,
		SortBy:  strings.TrimSpace(defaultSort),
		Order:   "asc",
		Filters: map[string]string{},
	}

	if query.SortBy == "" {
		for key := range allowedSort {
			query.SortBy = key
			break
		}
	}

	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			return query, fmt.Errorf("invalid limit")
		}
		if limit > maxIntegrationLimit {
			limit = maxIntegrationLimit
		}
		query.Limit = limit
	}

	if raw := strings.TrimSpace(values.Get("offset")); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return query, fmt.Errorf("invalid offset")
		}
		query.Offset = offset
	}

	if raw := strings.ToLower(strings.TrimSpace(values.Get("order"))); raw != "" {
		if raw != "asc" && raw != "desc" {
			return query, fmt.Errorf("invalid order")
		}
		query.Order = raw
	}

	if raw := strings.TrimSpace(values.Get("sort_by")); raw != "" {
		if _, ok := allowedSort[raw]; !ok {
			return query, fmt.Errorf("invalid sort_by")
		}
		query.SortBy = raw
	}

	allowedFilterSet := map[string]struct{}{}
	for _, item := range allowedFilters {
		allowedFilterSet[item] = struct{}{}
	}
	for key, rawValues := range values {
		if key == "limit" || key == "offset" || key == "sort_by" || key == "order" {
			continue
		}
		if _, ok := allowedFilterSet[key]; !ok {
			return query, fmt.Errorf("invalid filter: %s", key)
		}
		if len(rawValues) == 0 {
			continue
		}
		value := strings.TrimSpace(rawValues[0])
		if value == "" {
			continue
		}
		query.Filters[key] = value
	}

	return query, nil
}

func sortClause(query integrationListQuery, allowedSort map[string]string) string {
	column, ok := allowedSort[query.SortBy]
	if !ok || strings.TrimSpace(column) == "" {
		for _, fallback := range allowedSort {
			column = fallback
			break
		}
	}
	if strings.TrimSpace(column) == "" {
		column = "id"
	}
	return fmt.Sprintf("%s %s", column, query.Order)
}

func queryContextString(query integrationListQuery) string {
	filterParts := make([]string, 0, len(query.Filters))
	for key, value := range query.Filters {
		if len(value) > 64 {
			value = value[:64]
		}
		filterParts = append(filterParts, fmt.Sprintf("%s=%s", key, value))
	}
	return fmt.Sprintf("limit=%d;offset=%d;sort_by=%s;order=%s;filters=%s",
		query.Limit,
		query.Offset,
		query.SortBy,
		query.Order,
		strings.Join(filterParts, ","),
	)
}

func integrationListResponse(items interface{}, total int64, query integrationListQuery) map[string]interface{} {
	return map[string]interface{}{
		"items":   items,
		"total":   total,
		"limit":   query.Limit,
		"offset":  query.Offset,
		"sort_by": query.SortBy,
		"order":   query.Order,
	}
}

func parseBoolQuery(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean")
	}
}

func parseListQueryFromRequest(r *http.Request, allowedSort map[string]string, defaultSort string, allowedFilters []string) (integrationListQuery, error) {
	return parseIntegrationListQuery(r.URL.Query(), allowedSort, defaultSort, allowedFilters)
}
