package service

import (
	"fmt"
	"log"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPagedLimit = 100
	maxPagedLimit     = 200
)

type markerViewportPage struct {
	Items      []dbmodel.Marker
	NextCursor *string
}

type schedulePage struct {
	Items      []dbmodel.Schedule
	NextCursor *string
}

type MarkerViewportFilter struct {
	West   float64
	South  float64
	East   float64
	North  float64
	Zoom   *int
	Cursor *string
	Limit  *int
}

type PagedScheduleFilter struct {
	Time     string
	Status   *string
	MarkerID *int
	Label    *string
	Search   *string
	From     *string
	To       *string
	Cursor   *string
	Limit    *int
}

func normalizePagedLimit(limit *int) int {
	if limit == nil || *limit <= 0 {
		return defaultPagedLimit
	}
	if *limit > maxPagedLimit {
		return maxPagedLimit
	}
	return *limit
}

func parseCursor(cursor *string) (uint, error) {
	if cursor == nil || strings.TrimSpace(*cursor) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(*cursor), 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return uint(parsed), nil
}

func GetViewportMarkersPage(params MarkerViewportFilter, requested []string, relation dbmodel.UserRelation) (*markerViewportPage, error) {
	if params.South > params.North {
		return nil, fmt.Errorf("south cannot be greater than north")
	}
	if params.West < -180 || params.West > 180 {
		return nil, fmt.Errorf("invalid west")
	}
	if params.East < -180 || params.East > 180 {
		return nil, fmt.Errorf("invalid east")
	}
	if params.South < -90 || params.South > 90 {
		return nil, fmt.Errorf("invalid south")
	}
	if params.North < -90 || params.North > 90 {
		return nil, fmt.Errorf("invalid north")
	}

	limit := normalizePagedLimit(params.Limit)
	cursor, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, err
	}

	current := time.Now().AddDate(0, 0, -1)
	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}
	if utils.StringInSlice("restaurant", requested) {
		query = query.Preload("RestaurantInfo")
	}

	query = query.Model(&dbmodel.Marker{}).
		Where("relation_id = ?", relation.ID).
		Where("status != ?", constant.Arrived).
		Where("to_time IS NULL OR (to_time IS NOT NULL AND to_time >= ?)", current.Format(time.RFC3339)).
		Where("latitude >= ? AND latitude <= ?", params.South, params.North)

	if params.West <= params.East {
		query = query.Where("longitude >= ? AND longitude <= ?", params.West, params.East)
	} else {
		query = query.Where("(longitude >= ? OR longitude <= ?)", params.West, params.East)
	}
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}

	items := make([]dbmodel.Marker, 0)
	if err := query.Order("id asc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(items) == limit {
		cursorValue := strconv.FormatUint(uint64(items[len(items)-1].ID), 10)
		nextCursor = &cursorValue
	}

	return &markerViewportPage{
		Items:      items,
		NextCursor: nextCursor,
	}, logPagedQueryStats("viewport_markers", len(items), nextCursor)
}

func GetPagedSchedules(params PagedScheduleFilter, requested []string, relation dbmodel.UserRelation) (*schedulePage, error) {
	limit := normalizePagedLimit(params.Limit)
	cursor, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, err
	}

	baseDay, err := time.Parse("2006-01-02", strings.TrimSpace(params.Time))
	if err != nil {
		return nil, fmt.Errorf("invalid time format, expected YYYY-MM-DD")
	}

	query := database.Connection
	if utils.StringInSlice("created_by", requested) {
		query = query.Preload("CreatedBy")
	}
	if utils.StringInSlice("updated_by", requested) {
		query = query.Preload("UpdatedBy")
	}
	if utils.StringInSlice("marker", requested) {
		query = query.Preload("SelectedMarker.RestaurantInfo")
	}
	if utils.StringInSlice("movie", requested) {
		query = query.Preload("SelectedMovie")
	}

	query = query.Model(&dbmodel.Schedule{}).
		Where("relation_id = ?", relation.ID).
		Where("selected_date >= ?", baseDay.Format(time.RFC3339))

	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	if params.MarkerID != nil && *params.MarkerID > 0 {
		query = query.Where("marker_id = ?", *params.MarkerID)
	}
	if params.Label != nil && strings.TrimSpace(*params.Label) != "" {
		query = query.Where("label ILIKE ?", "%"+strings.TrimSpace(*params.Label)+"%")
	}
	if params.Search != nil && strings.TrimSpace(*params.Search) != "" {
		keyword := "%" + strings.TrimSpace(*params.Search) + "%"
		query = query.Where("label ILIKE ? OR description ILIKE ?", keyword, keyword)
	}
	if params.From != nil && strings.TrimSpace(*params.From) != "" {
		fromTime, convErr := time.Parse(time.RFC3339, strings.TrimSpace(*params.From))
		if convErr != nil {
			return nil, fmt.Errorf("invalid from, expected RFC3339")
		}
		query = query.Where("selected_date >= ?", fromTime.Format(time.RFC3339))
	}
	if params.To != nil && strings.TrimSpace(*params.To) != "" {
		toTime, convErr := time.Parse(time.RFC3339, strings.TrimSpace(*params.To))
		if convErr != nil {
			return nil, fmt.Errorf("invalid to, expected RFC3339")
		}
		query = query.Where("selected_date <= ?", toTime.Format(time.RFC3339))
	}
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}

	items := make([]dbmodel.Schedule, 0)
	if err := query.Order("id asc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}

	var nextCursor *string
	if len(items) == limit {
		cursorValue := strconv.FormatUint(uint64(items[len(items)-1].ID), 10)
		nextCursor = &cursorValue
	}

	return &schedulePage{
		Items:      items,
		NextCursor: nextCursor,
	}, logPagedQueryStats("paged_schedules", len(items), nextCursor)
}

func logPagedQueryStats(metric string, itemCount int, nextCursor *string) error {
	next := ""
	if nextCursor != nil {
		next = *nextCursor
	}
	log.Printf("paged_query metric=%s item_count=%d next_cursor=%q", metric, itemCount, next)
	return nil
}
