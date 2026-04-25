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
	Cursor  uint
	SortBy  string
	Order   string
	Filters map[string]string
}

type integrationPagedListQuery struct {
	Page    int
	PerPage int
	SortBy  string
	Order   string
	Filters map[string]string
}

func defaultIntegrationSort(allowedSort map[string]string, defaultSort string) string {
	sortBy := strings.TrimSpace(defaultSort)
	if sortBy != "" {
		return sortBy
	}
	for key := range allowedSort {
		return key
	}
	return ""
}

func parseIntegrationFilters(values url.Values, allowedFilters []string, reservedKeys map[string]struct{}) (map[string]string, error) {
	filters := map[string]string{}
	allowedFilterSet := map[string]struct{}{}
	for _, item := range allowedFilters {
		allowedFilterSet[item] = struct{}{}
	}
	for key, rawValues := range values {
		if _, ok := reservedKeys[key]; ok {
			continue
		}
		if _, ok := allowedFilterSet[key]; !ok {
			return nil, fmt.Errorf("invalid filter: %s", key)
		}
		if len(rawValues) == 0 {
			continue
		}
		value := strings.TrimSpace(rawValues[0])
		if key == "search" && len(rawValues) > 1 {
			searchValues := make([]string, 0, len(rawValues))
			for _, rawValue := range rawValues {
				trimmed := strings.TrimSpace(rawValue)
				if trimmed != "" {
					searchValues = append(searchValues, trimmed)
				}
			}
			value = strings.Join(searchValues, ",")
		}
		if value == "" {
			continue
		}
		filters[key] = value
	}
	return filters, nil
}

func parseIntegrationListQuery(values url.Values, allowedSort map[string]string, defaultSort string, allowedFilters []string) (integrationListQuery, error) {
	query := integrationListQuery{
		Limit:   defaultIntegrationLimit,
		Offset:  0,
		SortBy:  defaultIntegrationSort(allowedSort, defaultSort),
		Order:   "asc",
		Filters: map[string]string{},
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

	if raw := strings.TrimSpace(values.Get("cursor")); raw != "" {
		cursor, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || cursor == 0 {
			return query, fmt.Errorf("invalid cursor")
		}
		query.Cursor = uint(cursor)
	}
	if query.Cursor > 0 && query.Offset > 0 {
		return query, fmt.Errorf("cannot combine cursor and offset")
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

	filters, err := parseIntegrationFilters(values, allowedFilters, map[string]struct{}{
		"limit":   {},
		"offset":  {},
		"cursor":  {},
		"sort_by": {},
		"order":   {},
	})
	if err != nil {
		return query, err
	}
	query.Filters = filters

	return query, nil
}

func parseIntegrationPagedListQuery(values url.Values, allowedSort map[string]string, defaultSort string, allowedFilters []string) (integrationPagedListQuery, error) {
	query := integrationPagedListQuery{
		Page:    1,
		PerPage: defaultIntegrationLimit,
		SortBy:  defaultIntegrationSort(allowedSort, defaultSort),
		Order:   "asc",
		Filters: map[string]string{},
	}

	if raw := strings.TrimSpace(values.Get("page")); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page <= 0 {
			return query, fmt.Errorf("invalid page")
		}
		query.Page = page
	}

	if raw := strings.TrimSpace(values.Get("per_page")); raw != "" {
		perPage, err := strconv.Atoi(raw)
		if err != nil || perPage <= 0 {
			return query, fmt.Errorf("invalid per_page")
		}
		if perPage > maxIntegrationLimit {
			perPage = maxIntegrationLimit
		}
		query.PerPage = perPage
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

	filters, err := parseIntegrationFilters(values, allowedFilters, map[string]struct{}{
		"page":     {},
		"per_page": {},
		"sort_by":  {},
		"order":    {},
	})
	if err != nil {
		return query, err
	}
	query.Filters = filters

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

func integrationListResponse(items interface{}, total int64, query integrationListQuery, nextCursor string) map[string]interface{} {
	return map[string]interface{}{
		"items":      items,
		"total":      total,
		"limit":      query.Limit,
		"offset":     query.Offset,
		"cursor":     query.Cursor,
		"nextCursor": nextCursor,
		"sort_by":    query.SortBy,
		"order":      query.Order,
	}
}

func integrationPagedListResponse(items interface{}, total int64, query integrationPagedListQuery) map[string]interface{} {
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(query.PerPage) - 1) / int64(query.PerPage))
	}
	return map[string]interface{}{
		"items":       items,
		"total":       total,
		"page":        query.Page,
		"per_page":    query.PerPage,
		"total_pages": totalPages,
		"sort_by":     query.SortBy,
		"order":       query.Order,
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

func parsePagedListQueryFromRequest(r *http.Request, allowedSort map[string]string, defaultSort string, allowedFilters []string) (integrationPagedListQuery, error) {
	return parseIntegrationPagedListQuery(r.URL.Query(), allowedSort, defaultSort, allowedFilters)
}
