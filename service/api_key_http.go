package service

import (
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph/model"
	"mapmarker/backend/helper"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
)

type createAPIKeyRequest struct {
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	RelationID  uint     `json:"relation_id"`
	ActorUserID uint     `json:"actor_user_id"`
	ExpiresAt   *string  `json:"expires_at"`
}

type apiKeyResponse struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	Scopes      []string   `json:"scopes"`
	Status      string     `json:"status"`
	RelationID  uint       `json:"relation_id"`
	ActorUserID uint       `json:"actor_user_id"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type createAPIKeyResponse struct {
	APIKey apiKeyResponse `json:"api_key"`
	Token  string         `json:"token"`
}

type apiKeyUserOptionResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type apiKeyRelationOptionResponse struct {
	ID        uint   `json:"id"`
	UserOneID uint   `json:"user_one_id"`
	UserOne   string `json:"user_one"`
	UserTwoID uint   `json:"user_two_id"`
	UserTwo   string `json:"user_two"`
	Display   string `json:"display"`
}

type integrationCreateMarkerRequest struct {
	Label        string  `json:"label"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Address      string  `json:"address"`
	Type         string  `json:"type"`
	ImageLink    *string `json:"image_link"`
	Link         *string `json:"link"`
	Description  *string `json:"description"`
	Permanent    *bool   `json:"permanent"`
	NeedBooking  *bool   `json:"need_booking"`
	ToTime       *string `json:"to_time"`
	FromTime     *string `json:"from_time"`
	EstimateTime *string `json:"estimate_time"`
	RestaurantID *int    `json:"restaurant_id"`
	Price        *string `json:"price"`
}

type integrationUpdateMarkerRequest struct {
	Label            *string `json:"label"`
	Address          *string `json:"address"`
	ImageLink        *string `json:"image_link"`
	NoImage          bool    `json:"no_image"`
	Link             *string `json:"link"`
	Type             *string `json:"type"`
	Description      *string `json:"description"`
	Permanent        *bool   `json:"permanent"`
	NeedBooking      *bool   `json:"need_booking"`
	ToTime           *string `json:"to_time"`
	FromTime         *string `json:"from_time"`
	EstimateTime     *string `json:"estimate_time"`
	RestaurantID     *int    `json:"restaurant_id"`
	RemoveRestaurant *bool   `json:"remove_restaurant"`
	Price            *string `json:"price"`
}

type integrationCreateScheduleRequest struct {
	Label        string `json:"label"`
	Description  string `json:"description"`
	SelectedTime string `json:"selected_time"`
	MarkerID     int    `json:"marker_id"`
}

type integrationUpdateStationRequest struct {
	MapName          string   `json:"map_name"`
	Identifier       string   `json:"identifier"`
	Active           *bool    `json:"active"`
	Label            *string  `json:"label"`
	StationLocalName *string  `json:"station_local_name"`
	PhotoX           *float64 `json:"photo_x"`
	PhotoY           *float64 `json:"photo_y"`
	MapX             *float64 `json:"map_x"`
	MapY             *float64 `json:"map_y"`
	LineInfo         *string  `json:"line_info"`
}

type integrationUpdateDefaultPinRequest struct {
	PinID *int `json:"pin_id"`
}

func CreateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	operator := currentUserFromRequest(r)
	if operator == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	if err := helper.IsAuthorize(*operator, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	request := createAPIKeyRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if request.ExpiresAt != nil && strings.TrimSpace(*request.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*request.ExpiresAt))
		if err != nil {
			http.Error(w, "invalid expires_at format, expected RFC3339", http.StatusBadRequest)
			return
		}
		expiresAt = &parsed
	}

	apiKey, token, err := CreateAPIKey(APIKeyCreateInput{
		Name:        request.Name,
		Scopes:      request.Scopes,
		RelationID:  request.RelationID,
		ActorUserID: request.ActorUserID,
		ExpiresAt:   expiresAt,
	}, operator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, createAPIKeyResponse{
		APIKey: formatAPIKey(apiKey),
		Token:  token,
	})
}

func ListAPIKeysHandler(w http.ResponseWriter, r *http.Request) {
	operator := currentUserFromRequest(r)
	if operator == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	if err := helper.IsAuthorize(*operator, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	keys, err := ListAPIKeys()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]apiKeyResponse, 0, len(keys))
	for _, item := range keys {
		copyItem := item
		result = append(result, formatAPIKey(&copyItem))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items": result,
	})
}

func ListAPIKeyOptionsHandler(w http.ResponseWriter, r *http.Request) {
	operator := currentUserFromRequest(r)
	if operator == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	if err := helper.IsAuthorize(*operator, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	users, relations, err := ListAPIKeyOptions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userResponse := make([]apiKeyUserOptionResponse, 0, len(users))
	for _, item := range users {
		userResponse = append(userResponse, apiKeyUserOptionResponse{
			ID:       item.ID,
			Username: item.Username,
			Role:     item.Role,
		})
	}

	relationResponse := make([]apiKeyRelationOptionResponse, 0, len(relations))
	for _, item := range relations {
		relationResponse = append(relationResponse, apiKeyRelationOptionResponse{
			ID:        item.ID,
			UserOneID: item.UserOneID,
			UserOne:   item.UserOne,
			UserTwoID: item.UserTwoID,
			UserTwo:   item.UserTwo,
			Display:   item.Display,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"users":     userResponse,
		"relations": relationResponse,
	})
}

func RevokeAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	operator := currentUserFromRequest(r)
	if operator == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	if err := helper.IsAuthorize(*operator, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	key, err := RevokeAPIKey(uint(id), operator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"api_key": formatAPIKey(key),
	})
}

func RotateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	operator := currentUserFromRequest(r)
	if operator == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	if err := helper.IsAuthorize(*operator, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	key, token, err := RotateAPIKey(uint(id), operator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, createAPIKeyResponse{
		APIKey: formatAPIKey(key),
		Token:  token,
	})
}

func DeleteAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	operator := currentUserFromRequest(r)
	if operator == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	if err := helper.IsAuthorize(*operator, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := DeleteAPIKey(uint(id)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deleted",
		"id":     id,
	})
}

func IntegrationListMarkersHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"label":      "label",
		"type":       "type",
		"to_time":    "to_time",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "updated_at", []string{"type", "status", "country", "country_code", "label", "search"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.markers.list", constant.APIKeyScopeMarkersRead, queryContextString(queryOption))
	if !ok {
		return
	}

	current := time.Now().AddDate(0, 0, -1)
	query := database.Connection.Model(&dbmodel.Marker{})
	query = query.Where("relation_id = ?", apiKey.Relation.ID)
	query = query.Where("status != ?", constant.Arrived)
	query = query.Where("to_time IS NULL OR (to_time IS NOT NULL AND to_time >= ?)", current.Format(time.RFC3339))

	if value, ok := queryOption.Filters["type"]; ok {
		query = query.Where("type = ?", value)
	}
	if value, ok := queryOption.Filters["status"]; ok {
		query = query.Where("status = ?", value)
	}
	if value, ok := queryOption.Filters["country"]; ok {
		query = query.Where("country = ?", value)
	}
	if value, ok := queryOption.Filters["country_code"]; ok {
		query = query.Where("country_code = ?", value)
	}
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("label ILIKE ?", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["search"]; ok {
		keyword := "%" + value + "%"
		query = query.Where("label ILIKE ? OR address ILIKE ? OR description ILIKE ?", keyword, keyword, keyword)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	markers := make([]dbmodel.Marker, 0)
	err = query.
		Order(sortClause(queryOption, allowedSort)).
		Limit(queryOption.Limit).
		Offset(queryOption.Offset).
		Find(&markers).Error
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]model.Marker, 0, len(markers))
	for _, item := range markers {
		response = append(response, helper.ConvertMarker(item))
	}
	respondJSON(w, http.StatusOK, integrationListResponse(response, total, queryOption))
}

func IntegrationCreateMarkerHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.markers.create", constant.APIKeyScopeMarkersWrite, "")
	if !ok {
		return
	}

	request := integrationCreateMarkerRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input := model.NewMarker{
		Label:        request.Label,
		Latitude:     request.Latitude,
		Longitude:    request.Longitude,
		Address:      request.Address,
		Type:         request.Type,
		ImageLink:    request.ImageLink,
		Link:         request.Link,
		Description:  request.Description,
		Permanent:    request.Permanent,
		NeedBooking:  request.NeedBooking,
		ToTime:       request.ToTime,
		FromTime:     request.FromTime,
		EstimateTime: request.EstimateTime,
		RestaurantID: request.RestaurantID,
		Price:        request.Price,
	}

	var restaurant dbmodel.Restaurant
	var restaurantPtr *dbmodel.Restaurant
	if request.RestaurantID != nil {
		restaurant.ID = uint(*request.RestaurantID)
		if err := restaurant.GetById(database.Connection); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		restaurantPtr = &restaurant
	}

	marker, err := CreateMarker(input, restaurantPtr, apiKey.ActorUser, apiKey.Relation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, helper.ConvertMarker(*marker))
}

func IntegrationUpdateMarkerHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.markers.update", constant.APIKeyScopeMarkersWrite, "")
	if !ok {
		return
	}

	markerID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || markerID <= 0 {
		http.Error(w, "invalid marker id", http.StatusBadRequest)
		return
	}

	request := integrationUpdateMarkerRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input := model.UpdateMarker{
		ID:               markerID,
		Label:            request.Label,
		Address:          request.Address,
		ImageLink:        request.ImageLink,
		NoImage:          request.NoImage,
		Link:             request.Link,
		Type:             request.Type,
		Description:      request.Description,
		Permanent:        request.Permanent,
		NeedBooking:      request.NeedBooking,
		ToTime:           request.ToTime,
		FromTime:         request.FromTime,
		EstimateTime:     request.EstimateTime,
		RestaurantID:     request.RestaurantID,
		RemoveRestaurant: request.RemoveRestaurant,
		Price:            request.Price,
	}

	var restaurant dbmodel.Restaurant
	var restaurantPtr *dbmodel.Restaurant
	if request.RestaurantID != nil {
		restaurant.ID = uint(*request.RestaurantID)
		if err := restaurant.GetById(database.Connection); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		restaurantPtr = &restaurant
	}

	marker, err := EditMarker(input, restaurantPtr, apiKey.Relation, apiKey.ActorUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, helper.ConvertMarker(*marker))
}

func IntegrationListSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"created_at":    "created_at",
		"updated_at":    "updated_at",
		"selected_date": "selected_date",
		"label":         "label",
		"status":        "status",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "selected_date", []string{"status", "marker_id", "label", "search", "from", "to", "time"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.schedules.list", constant.APIKeyScopeSchedulesRead, queryContextString(queryOption))
	if !ok {
		return
	}

	day := strings.TrimSpace(queryOption.Filters["time"])
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}

	baseDay, err := time.Parse("2006-01-02", day)
	if err != nil {
		http.Error(w, "invalid time format, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	query := database.Connection.Model(&dbmodel.Schedule{}).Preload("SelectedMarker.RestaurantInfo").Where("relation_id = ?", apiKey.Relation.ID)
	query = query.Where("selected_date >= ?", baseDay.Format(time.RFC3339))
	if value, ok := queryOption.Filters["status"]; ok {
		query = query.Where("status = ?", value)
	}
	if value, ok := queryOption.Filters["marker_id"]; ok {
		markerID, convErr := strconv.Atoi(value)
		if convErr != nil || markerID <= 0 {
			http.Error(w, "invalid marker_id", http.StatusBadRequest)
			return
		}
		query = query.Where("marker_id = ?", markerID)
	}
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("label ILIKE ?", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["search"]; ok {
		keyword := "%" + value + "%"
		query = query.Where("label ILIKE ? OR description ILIKE ?", keyword, keyword)
	}
	if value, ok := queryOption.Filters["from"]; ok {
		fromTime, convErr := time.Parse(time.RFC3339, value)
		if convErr != nil {
			http.Error(w, "invalid from, expected RFC3339", http.StatusBadRequest)
			return
		}
		query = query.Where("selected_date >= ?", fromTime.Format(time.RFC3339))
	}
	if value, ok := queryOption.Filters["to"]; ok {
		toTime, convErr := time.Parse(time.RFC3339, value)
		if convErr != nil {
			http.Error(w, "invalid to, expected RFC3339", http.StatusBadRequest)
			return
		}
		query = query.Where("selected_date <= ?", toTime.Format(time.RFC3339))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	schedules := make([]dbmodel.Schedule, 0)
	err = query.
		Order(sortClause(queryOption, allowedSort)).
		Limit(queryOption.Limit).
		Offset(queryOption.Offset).
		Find(&schedules).Error
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := make([]model.Schedule, 0, len(schedules))
	for _, item := range schedules {
		response = append(response, helper.ConvertSchedule(item))
	}
	respondJSON(w, http.StatusOK, integrationListResponse(response, total, queryOption))
}

func IntegrationCreateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.schedules.create", constant.APIKeyScopeSchedulesWrite, "")
	if !ok {
		return
	}

	request := integrationCreateScheduleRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var marker dbmodel.Marker
	marker.ID = uint(request.MarkerID)
	if err := marker.GetById(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if marker.RelationId != apiKey.Relation.ID {
		http.Error(w, "api key cannot access marker outside assigned relation", http.StatusForbidden)
		return
	}

	input := model.NewSchedule{
		Label:        request.Label,
		Description:  request.Description,
		SelectedTime: request.SelectedTime,
		MarkerID:     request.MarkerID,
	}

	tx := database.Connection.Begin()
	schedule, err := CreateSchedule(tx, input, marker, apiKey.ActorUser, apiKey.Relation)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, helper.ConvertSchedule(*schedule))
}

func IntegrationListStationsHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"label":      "ts.label",
		"identifier": "ts.identifier",
		"map_name":   "ts.map_name",
		"active":     "COALESCE(tr.active, false)",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "label", []string{"map_name", "identifier", "label", "active"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.stations.list", constant.APIKeyScopeStationsRead, queryContextString(queryOption))
	if !ok {
		return
	}

	type stationRow struct {
		Label            string  `gorm:"column:label"`
		StationLocalName string  `gorm:"column:station_local_name"`
		Identifier       string  `gorm:"column:identifier"`
		PhotoX           float64 `gorm:"column:photo_x"`
		PhotoY           float64 `gorm:"column:photo_y"`
		MapX             float64 `gorm:"column:map_x"`
		MapY             float64 `gorm:"column:map_y"`
		LineInfo         string  `gorm:"column:line_info"`
		MapName          string  `gorm:"column:map_name"`
		Active           bool    `gorm:"column:active"`
	}

	query := database.Connection.Table("train_stations ts").
		Select("ts.*, COALESCE(tr.active, false) as active").
		Joins("LEFT JOIN train_records tr ON tr.station_id = ts.id AND tr.relation_id = ?", apiKey.Relation.ID)

	if value, ok := queryOption.Filters["map_name"]; ok {
		query = query.Where("ts.map_name = ?", value)
	}
	if value, ok := queryOption.Filters["identifier"]; ok {
		query = query.Where("ts.identifier = ?", value)
	}
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("ts.label ILIKE ? OR ts.station_local_name ILIKE ?", "%"+value+"%", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["active"]; ok {
		active, convErr := parseBoolQuery(value)
		if convErr != nil {
			http.Error(w, "invalid active", http.StatusBadRequest)
			return
		}
		query = query.Where("COALESCE(tr.active, false) = ?", active)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]stationRow, 0)
	if err := query.Order(sortClause(queryOption, allowedSort)).Limit(queryOption.Limit).Offset(queryOption.Offset).Scan(&rows).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]model.Station, 0, len(rows))
	for _, row := range rows {
		response = append(response, model.Station{
			Label:      row.Label,
			LocalName:  row.StationLocalName,
			Identifier: row.Identifier,
			PhotoX:     row.PhotoX,
			PhotoY:     row.PhotoY,
			MapX:       row.MapX,
			MapY:       row.MapY,
			Active:     row.Active,
			MapName:    row.MapName,
			LineInfo:   row.LineInfo,
		})
	}

	respondJSON(w, http.StatusOK, integrationListResponse(response, total, queryOption))
}

func IntegrationUpdateStationHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.stations.update", constant.APIKeyScopeStationsWrite, "")
	if !ok {
		return
	}

	request := integrationUpdateStationRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	request.MapName = strings.TrimSpace(request.MapName)
	request.Identifier = strings.TrimSpace(request.Identifier)
	if request.MapName == "" || request.Identifier == "" {
		http.Error(w, "map_name and identifier are required", http.StatusBadRequest)
		return
	}

	var station dbmodel.TrainStation
	station.Identifier = request.Identifier
	station.MapName = request.MapName
	if err := station.GetByMapAndIdentifier(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.Label != nil {
		station.Label = strings.TrimSpace(*request.Label)
	}
	if request.StationLocalName != nil {
		station.StationLocalName = strings.TrimSpace(*request.StationLocalName)
	}
	if request.PhotoX != nil {
		station.PhotoX = *request.PhotoX
	}
	if request.PhotoY != nil {
		station.PhotoY = *request.PhotoY
	}
	if request.MapX != nil {
		station.MapX = *request.MapX
	}
	if request.MapY != nil {
		station.MapY = *request.MapY
	}
	if request.LineInfo != nil {
		trimmed := strings.TrimSpace(*request.LineInfo)
		if trimmed != "" && !json.Valid([]byte(trimmed)) {
			http.Error(w, "line_info must be valid JSON", http.StatusBadRequest)
			return
		}
		if trimmed == "" {
			trimmed = "[]"
		}
		station.LineInfo = trimmed
	}
	if err := station.Update(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if request.LineInfo != nil {
		lines, err := parseLineInfoJSON(station.LineInfo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updatedStation, err := UpdateTrainStationLines(station.MapName, station.Identifier, lines)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		station = *updatedStation
	}

	active := false
	if request.Active != nil {
		var record dbmodel.TrainRecord
		record.SelectedStation = station
		record.Relation = apiKey.Relation
		tx := database.Connection.Begin()
		if err := record.GetOrCreate(tx); err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		record.Active = *request.Active
		record.UpdatedBy = &apiKey.ActorUser
		if err := record.Update(tx); err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := tx.Commit().Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		active = record.Active
	} else {
		var record dbmodel.TrainRecord
		if err := database.Connection.Where("station_id = ? AND relation_id = ?", station.ID, apiKey.Relation.ID).First(&record).Error; err == nil {
			active = record.Active
		}
	}

	output := helper.ConvertTrainStation(station)
	output.Active = active
	respondJSON(w, http.StatusOK, output)
}

func IntegrationListSettingsPinsHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"label":      "label",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "label", []string{"label"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, ok := authenticateIntegrationRequest(w, r, "integration.settings.pins.list", constant.APIKeyScopeSettingsRead, queryContextString(queryOption))
	if !ok {
		return
	}

	query := database.Connection.Model(&dbmodel.Pin{})
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("label ILIKE ?", "%"+value+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]dbmodel.Pin, 0)
	if err := query.Order(sortClause(queryOption, allowedSort)).Limit(queryOption.Limit).Offset(queryOption.Offset).Find(&items).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption))
}

func IntegrationListSettingsMarkerTypesHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"label":      "label",
		"priority":   "priority",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "priority", []string{"label", "value", "hidden"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, ok := authenticateIntegrationRequest(w, r, "integration.settings.marker_types.list", constant.APIKeyScopeSettingsRead, queryContextString(queryOption))
	if !ok {
		return
	}

	query := database.Connection.Model(&dbmodel.MarkerType{})
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("label ILIKE ?", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["value"]; ok {
		query = query.Where("value ILIKE ?", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["hidden"]; ok {
		hidden, convErr := parseBoolQuery(value)
		if convErr != nil {
			http.Error(w, "invalid hidden", http.StatusBadRequest)
			return
		}
		query = query.Where("hidden = ?", hidden)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]dbmodel.MarkerType, 0)
	if err := query.Order(sortClause(queryOption, allowedSort)).Limit(queryOption.Limit).Offset(queryOption.Offset).Find(&items).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption))
}

func IntegrationListSettingsDefaultPinsHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"label":      "default_values.label",
		"created_at": "default_values.created_at",
		"updated_at": "default_values.updated_at",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "label", []string{"label"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, ok := authenticateIntegrationRequest(w, r, "integration.settings.default_pins.list", constant.APIKeyScopeSettingsRead, queryContextString(queryOption))
	if !ok {
		return
	}

	query := database.Connection.Model(&dbmodel.DefaultValue{}).Preload("PinType")
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("default_values.label ILIKE ?", "%"+value+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]dbmodel.DefaultValue, 0)
	if err := query.Order(sortClause(queryOption, allowedSort)).Limit(queryOption.Limit).Offset(queryOption.Offset).Find(&items).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption))
}

func IntegrationUpdateSettingsDefaultPinHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.settings.default_pins.update", constant.APIKeyScopeSettingsWrite, "")
	if !ok {
		return
	}

	label := strings.TrimSpace(chi.URLParam(r, "label"))
	if label == "" {
		http.Error(w, "label is required", http.StatusBadRequest)
		return
	}

	request := integrationUpdateDefaultPinRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.PinID == nil || *request.PinID <= 0 {
		http.Error(w, "pin_id is required", http.StatusBadRequest)
		return
	}

	input := model.UpdatedDefault{
		Label:       label,
		UpdatedType: "int",
		IntValue:    request.PinID,
	}
	if _, err := EditDefaultPin(input, apiKey.ActorUser); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var updated dbmodel.DefaultValue
	updated.Label = label
	if err := updated.GetOrCreatePin(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func authenticateIntegrationRequest(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
	operationWithContext := strings.TrimSpace(operation)
	if strings.TrimSpace(queryContext) != "" {
		operationWithContext = operationWithContext + " [" + strings.TrimSpace(queryContext) + "]"
	}

	raw := strings.TrimSpace(r.Header.Get("X-API-Key"))
	if raw == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "apikey ") {
			raw = strings.TrimSpace(authHeader[7:])
		}
	}

	sourceIP := requestSourceIP(r)
	if raw == "" {
		_ = CreateAPIKeyAuditLog(nil, "unknown", APIKeyAuditEvent{
			Operation: operationWithContext,
			SourceIP:  sourceIP,
			Success:   false,
			Reason:    "missing api key header",
		})
		if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
			http.Error(w, "jwt-based automation auth is deprecated for integration endpoints; use API key", http.StatusUnauthorized)
			return nil, false
		}
		http.Error(w, "missing api key", http.StatusUnauthorized)
		return nil, false
	}

	apiKey, err := AuthenticateAPIKey(raw, APIKeyAuditEvent{
		Operation: operationWithContext,
		SourceIP:  sourceIP,
		Success:   false,
		Reason:    "authentication failed",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return nil, false
	}

	if !HasScope(apiKey, requiredScope) {
		_ = CreateAPIKeyAuditLog(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
			Operation: operationWithContext,
			SourceIP:  sourceIP,
			Success:   false,
			Reason:    fmt.Sprintf("missing scope: %s", requiredScope),
		})
		http.Error(w, (&helper.APIKeyScopeDeniedError{}).Error(), http.StatusForbidden)
		return nil, false
	}

	return apiKey, true
}

func formatAPIKey(apiKey *dbmodel.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID:          apiKey.ID,
		Name:        apiKey.Name,
		Prefix:      apiKey.Prefix,
		Scopes:      apiKey.ScopeList(),
		Status:      apiKey.Status,
		RelationID:  apiKey.RelationID,
		ActorUserID: apiKey.ActorUserID,
		LastUsedAt:  apiKey.LastUsedAt,
		ExpiresAt:   apiKey.ExpiresAt,
		CreatedAt:   apiKey.CreatedAt,
	}
}

func requestSourceIP(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return strings.TrimSpace(host)
}

func currentUserFromRequest(r *http.Request) *dbmodel.User {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return nil
	}

	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		header = strings.TrimSpace(header[7:])
	}

	user, err := ValidateToken(header)
	if err != nil {
		log.Printf("failed to validate current user from request: %v", err)
		return nil
	}
	return user
}
