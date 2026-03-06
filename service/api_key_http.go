package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type integrationCreateMarkerOutcomeRequest struct {
	Link           string  `json:"link"`
	Status         string  `json:"status"`
	MarkerID       *uint   `json:"markerId"`
	ExternalRunID  *string `json:"externalRunId"`
	FailureReason  *string `json:"failureReason"`
	FailureMessage *string `json:"failureMessage"`
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

type updateOwnPreviewPinRequest struct {
	PinID *int `json:"pin_id"`
}

type integrationUpdateUserPreviewPinRequest struct {
	PinID *int `json:"pin_id"`
}

type integrationUserPreviewPinResponse struct {
	Username string `json:"username"`
	PinID    *uint  `json:"pin_id,omitempty"`
	PinLabel string `json:"pin_label,omitempty"`
}

type integrationStaticPreviewRequest struct {
	Username       string   `json:"username"`
	MarkerTypeName string   `json:"marker_type_name,omitempty"`
	Lat            *float64 `json:"lat,omitempty"`
	Lon            *float64 `json:"lon,omitempty"`
}

type integrationStaticPreviewResponse struct {
	Username    string  `json:"username"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	ImageBase64 string  `json:"image_base64"`
	MimeType    string  `json:"mime_type"`
	Format      string  `json:"format"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
}

type integrationStaticPreviewGeocodeRequest struct {
	StreetNumber string `json:"street_number"`
	StreetName   string `json:"street_name"`
	Country      string `json:"country"`
}

type integrationStaticPreviewGeocodeResponse struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type integrationMarkerResponse struct {
	model.Marker
	IntegrationSource       string `json:"integration_source"`
	CreatedAgo              string `json:"created_ago"`
	EditableWithSameAPIKey  bool   `json:"editable_with_same_api_key"`
	RemovableWithSameAPIKey bool   `json:"removable_with_same_api_key"`
}

type integrationMarkerOutcomeResponse struct {
	ID             uint      `json:"id"`
	Link           string    `json:"link"`
	Status         string    `json:"status"`
	MarkerID       *uint     `json:"markerId,omitempty"`
	ExternalRunID  *string   `json:"externalRunId,omitempty"`
	FailureReason  *string   `json:"failureReason,omitempty"`
	FailureMessage *string   `json:"failureMessage,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type integrationErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var integrationGetUserPreviewPinFn = GetUserPreviewPinSelection
var integrationSetUserPreviewPinFn = SetUserPreviewPinSelection
var integrationGenerateStaticMapFn = GenerateStaticMapPreviewByUsername
var integrationGeocodeAddressFn = GeocodeStreetAddress
var integrationAuthenticateRequestFn = authenticateIntegrationRequest
var integrationCreateAuditLogFn = CreateAPIKeyAuditLog
var integrationCreateMarkerOutcomeLogFn = CreateMarkerCreationOutcomeLog
var integrationNowFn = time.Now
var integrationGetMarkerByIDFn = func(id uint) (*dbmodel.Marker, error) {
	marker := &dbmodel.Marker{}
	marker.ID = id
	if err := marker.GetById(database.Connection); err != nil {
		return nil, err
	}
	return marker, nil
}
var integrationUpdateMarkerModelFn = func(marker *dbmodel.Marker) error {
	return marker.Update(database.Connection)
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
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "updated_at", []string{"type", "status", "country", "country_code", "label", "search", "west", "south", "east", "north", "zoom"})
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

	westRaw, hasWest := queryOption.Filters["west"]
	southRaw, hasSouth := queryOption.Filters["south"]
	eastRaw, hasEast := queryOption.Filters["east"]
	northRaw, hasNorth := queryOption.Filters["north"]
	if hasWest || hasSouth || hasEast || hasNorth {
		if !hasWest || !hasSouth || !hasEast || !hasNorth {
			http.Error(w, "bbox requires west,south,east,north", http.StatusBadRequest)
			return
		}

		west, convErr := strconv.ParseFloat(westRaw, 64)
		if convErr != nil || west < -180 || west > 180 {
			http.Error(w, "invalid west", http.StatusBadRequest)
			return
		}
		south, convErr := strconv.ParseFloat(southRaw, 64)
		if convErr != nil || south < -90 || south > 90 {
			http.Error(w, "invalid south", http.StatusBadRequest)
			return
		}
		east, convErr := strconv.ParseFloat(eastRaw, 64)
		if convErr != nil || east < -180 || east > 180 {
			http.Error(w, "invalid east", http.StatusBadRequest)
			return
		}
		north, convErr := strconv.ParseFloat(northRaw, 64)
		if convErr != nil || north < -90 || north > 90 {
			http.Error(w, "invalid north", http.StatusBadRequest)
			return
		}
		if south > north {
			http.Error(w, "south cannot be greater than north", http.StatusBadRequest)
			return
		}

		query = query.Where("latitude >= ? AND latitude <= ?", south, north)
		if west <= east {
			query = query.Where("longitude >= ? AND longitude <= ?", west, east)
		} else {
			// Crossing the antimeridian; match either edge slice.
			query = query.Where("(longitude >= ? OR longitude <= ?)", west, east)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	markers := make([]dbmodel.Marker, 0)
	queryWithSort := query
	if queryOption.Cursor > 0 {
		queryWithSort = queryWithSort.Where("id > ?", queryOption.Cursor)
		queryWithSort = queryWithSort.Order("id asc")
	} else {
		queryWithSort = queryWithSort.Order(sortClause(queryOption, allowedSort)).Order("id asc")
	}
	err = queryWithSort.
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
	decorated := make([]integrationMarkerResponse, 0, len(response))
	for _, marker := range response {
		decorated = append(decorated, buildIntegrationMarkerResponse(marker))
	}
	nextCursor := ""
	if len(markers) == queryOption.Limit {
		nextCursor = strconv.FormatUint(uint64(markers[len(markers)-1].ID), 10)
	}
	respondJSON(w, http.StatusOK, integrationListResponse(decorated, total, queryOption, nextCursor))
}

func IntegrationCreateMarkerHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.markers.create", constant.APIKeyScopeMarkersWrite, "")
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
	if err := validateCoordinates(input.Latitude, input.Longitude); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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

	respondJSON(w, http.StatusCreated, buildIntegrationMarkerResponse(helper.ConvertMarker(*marker)))
}

func IntegrationCreateMarkerOutcomeHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := integrationAuthenticateRequestFn(w, r, "integration.markers.outcomes.create", constant.APIKeyScopeMarkersWrite, "")
	if !ok {
		return
	}

	requestPayload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request := integrationCreateMarkerOutcomeRequest{}
	if err := decodeStrictJSONPayload(requestPayload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request.Link = strings.TrimSpace(request.Link)
	request.Status = strings.ToLower(strings.TrimSpace(request.Status))
	if request.Link == "" {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_link", "link is required")
		return
	}
	if !isValidMarkerOutcomeStatus(request.Status) {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_status", "status must be one of: success, failed")
		return
	}
	if request.Status == dbmodel.MarkerCreationOutcomeStatusSuccess && request.MarkerID == nil && strings.TrimSpace(optionalStringValue(request.ExternalRunID)) == "" {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_success_reference", "success status requires markerId or externalRunId")
		return
	}

	item, err := integrationCreateMarkerOutcomeLogFn(MarkerCreationOutcomeLogCreateInput{
		Link:           request.Link,
		Status:         request.Status,
		MarkerID:       request.MarkerID,
		ExternalRunID:  request.ExternalRunID,
		FailureReason:  request.FailureReason,
		FailureMessage: request.FailureMessage,
	})
	if err != nil {
		writeIntegrationError(w, http.StatusInternalServerError, "outcome_log_persist_failed", "failed to persist marker creation outcome log")
		return
	}

	respondJSON(w, http.StatusCreated, integrationMarkerOutcomeResponse{
		ID:             item.ID,
		Link:           item.Link,
		Status:         item.Status,
		MarkerID:       item.MarkerID,
		ExternalRunID:  item.ExternalRunID,
		FailureReason:  item.FailureReason,
		FailureMessage: item.FailureMessage,
		CreatedAt:      item.CreatedAt,
	})
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

	respondJSON(w, http.StatusOK, buildIntegrationMarkerResponse(helper.ConvertMarker(*marker)))
}

func IntegrationDeleteMarkerHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.markers.delete", constant.APIKeyScopeMarkersWrite, "")
	if !ok {
		return
	}

	markerID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || markerID <= 0 {
		http.Error(w, "invalid marker id", http.StatusBadRequest)
		return
	}

	marker, err := integrationGetMarkerByIDFn(uint(markerID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if marker.RelationId != apiKey.Relation.ID {
		http.Error(w, "api key cannot delete marker outside assigned relation", http.StatusForbidden)
		return
	}

	marker.Status = constant.Cancelled
	marker.UpdatedBy = &apiKey.ActorUser
	if err := integrationUpdateMarkerModelFn(marker); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":     markerID,
		"status": "cancelled",
	})
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
	queryWithSort := query
	if queryOption.Cursor > 0 {
		queryWithSort = queryWithSort.Where("id > ?", queryOption.Cursor)
		queryWithSort = queryWithSort.Order("id asc")
	} else {
		queryWithSort = queryWithSort.Order(sortClause(queryOption, allowedSort)).Order("id asc")
	}
	err = queryWithSort.
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
	nextCursor := ""
	if len(schedules) == queryOption.Limit {
		nextCursor = strconv.FormatUint(uint64(schedules[len(schedules)-1].ID), 10)
	}
	respondJSON(w, http.StatusOK, integrationListResponse(response, total, queryOption, nextCursor))
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

	respondJSON(w, http.StatusOK, integrationListResponse(response, total, queryOption, ""))
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

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption, ""))
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

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption, ""))
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

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption, ""))
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

func SettingsGetPreviewPinHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	_, preference, err := GetUserPreviewPinSelection(user.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := integrationUserPreviewPinResponse{
		Username: user.Username,
	}
	if preference != nil && preference.PreviewPinID != nil {
		response.PinID = preference.PreviewPinID
		if preference.PreviewPin != nil {
			response.PinLabel = preference.PreviewPin.Label
		}
	}

	respondJSON(w, http.StatusOK, response)
}

func SettingsUpdatePreviewPinHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	request := updateOwnPreviewPinRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.PinID == nil || *request.PinID <= 0 {
		http.Error(w, "pin_id is required", http.StatusBadRequest)
		return
	}

	preference, pin, err := SetUserPreviewPinSelection(user.Username, uint(*request.PinID), user)
	if err != nil {
		if err == ErrPreviewPinInvalid {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_preview_pin", "pin_id must reference an existing pin")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, integrationUserPreviewPinResponse{
		Username: user.Username,
		PinID:    preference.PreviewPinID,
		PinLabel: pin.Label,
	})
}

func IntegrationGetUserPreviewPinSelectionHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := integrationAuthenticateRequestFn(w, r, "integration.settings.preview_pin.get", constant.APIKeyScopeSettingsRead, "")
	if !ok {
		return
	}

	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	user, preference, err := integrationGetUserPreviewPinFn(username)
	if err != nil {
		if err == ErrUnknownUsername {
			writeIntegrationError(w, http.StatusNotFound, "unknown_username", "username does not exist")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := integrationUserPreviewPinResponse{
		Username: user.Username,
	}
	if preference != nil && preference.PreviewPinID != nil {
		response.PinID = preference.PreviewPinID
		if preference.PreviewPin != nil {
			response.PinLabel = preference.PreviewPin.Label
		}
	}

	respondJSON(w, http.StatusOK, response)
}

func IntegrationUpdateUserPreviewPinSelectionHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.settings.preview_pin.update", constant.APIKeyScopeSettingsWrite, "")
	if !ok {
		return
	}

	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	request := integrationUpdateUserPreviewPinRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.PinID == nil || *request.PinID <= 0 {
		http.Error(w, "pin_id is required", http.StatusBadRequest)
		return
	}

	preference, pin, err := integrationSetUserPreviewPinFn(username, uint(*request.PinID), &apiKey.ActorUser)
	if err != nil {
		switch err {
		case ErrUnknownUsername:
			writeIntegrationError(w, http.StatusNotFound, "unknown_username", "username does not exist")
		case ErrPreviewPinInvalid:
			writeIntegrationError(w, http.StatusBadRequest, "invalid_preview_pin", "pin_id must reference an existing active pin")
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	respondJSON(w, http.StatusOK, integrationUserPreviewPinResponse{
		Username: username,
		PinID:    preference.PreviewPinID,
		PinLabel: pin.Label,
	})
}

func IntegrationGenerateStaticMapPreviewHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.static_preview.generate", constant.APIKeyScopeStaticPreview, "")
	if !ok {
		return
	}

	requestPayload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request := integrationStaticPreviewRequest{}
	if err := decodeStrictJSONPayload(requestPayload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.MarkerTypeName = strings.TrimSpace(request.MarkerTypeName)
	if request.Username == "" {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_username", "username is required")
		return
	}
	if request.Lat == nil || request.Lon == nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_location_input", "lat and lon are required")
		return
	}

	lat := *request.Lat
	lon := *request.Lon

	result, err := integrationGenerateStaticMapFn(request.Username, request.MarkerTypeName, lat, lon)
	if err != nil {
		recordStaticPreviewFailure(apiKey, requestSourceIP(r), err.Error(), request.Username)
		switch err {
		case ErrUnknownUsername:
			writeIntegrationError(w, http.StatusNotFound, "unknown_username", "username does not exist")
		case ErrMarkerTypeNotFound:
			writeIntegrationError(w, http.StatusBadRequest, "invalid_marker_type_input", "marker_type_name does not match an existing marker type")
		case ErrPreviewPinSelectionRequired:
			writeIntegrationError(w, http.StatusBadRequest, "preview_pin_not_configured", "preview pin selection is required for this user")
		case ErrPreviewPinInvalid:
			writeIntegrationError(w, http.StatusBadRequest, "invalid_preview_pin", "configured preview pin is invalid")
		default:
			if errors.Is(err, ErrInvalidCoordinates) {
				writeIntegrationError(w, http.StatusBadRequest, "invalid_coordinates", "lat and lon must be within valid ranges")
				return
			}
			if errors.Is(err, ErrTomTomStaticMap) {
				writeIntegrationError(w, http.StatusBadGateway, "tomtom_dependency_failure", "failed to retrieve static map from TomTom")
				return
			}
			if errors.Is(err, ErrImageComposition) {
				writeIntegrationError(w, http.StatusInternalServerError, "image_composition_failure", "failed to compose preview image")
				return
			}
			writeIntegrationError(w, http.StatusInternalServerError, "preview_generation_failed", "failed to generate static preview")
		}
		return
	}

	_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
		Operation: "integration.static_preview.generate.result",
		SourceIP:  requestSourceIP(r),
		Success:   true,
		Reason:    fmt.Sprintf("username=%s", result.User.Username),
	})

	respondJSON(w, http.StatusOK, integrationStaticPreviewResponse{
		Username:    result.User.Username,
		Lat:         lat,
		Lon:         lon,
		ImageBase64: StaticPreviewToBase64(result.Image),
		MimeType:    result.MimeType,
		Format:      result.Format,
		Width:       result.Width,
		Height:      result.Height,
	})
}

func IntegrationGeocodeStaticMapPreviewHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.static_preview.geocode", constant.APIKeyScopeStaticPreview, "")
	if !ok {
		return
	}

	requestPayload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request := integrationStaticPreviewGeocodeRequest{}
	if err := decodeStrictJSONPayload(requestPayload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request.StreetNumber = strings.TrimSpace(request.StreetNumber)
	request.StreetName = strings.TrimSpace(request.StreetName)
	request.Country = strings.TrimSpace(request.Country)
	if request.StreetNumber == "" || request.StreetName == "" || request.Country == "" {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_address_input", "street_number, street_name, and country are required")
		return
	}

	lat, lon, geocodeErr := integrationGeocodeAddressFn(request.StreetNumber, request.StreetName, request.Country)
	if geocodeErr != nil {
		_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
			Operation: "integration.static_preview.geocode.result",
			SourceIP:  requestSourceIP(r),
			Success:   false,
			Reason:    geocodeErr.Error(),
		})
		writeIntegrationError(w, http.StatusBadGateway, "geocode_dependency_failure", "failed to geocode address to coordinates")
		return
	}

	_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
		Operation: "integration.static_preview.geocode.result",
		SourceIP:  requestSourceIP(r),
		Success:   true,
		Reason:    fmt.Sprintf("lat=%f; lon=%f", lat, lon),
	})

	respondJSON(w, http.StatusOK, integrationStaticPreviewGeocodeResponse{
		Lat: lat,
		Lon: lon,
	})
}

func readJSONBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	return body, nil
}

func decodeStrictJSONPayload(payload []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return nil
}

func buildIntegrationMarkerResponse(marker model.Marker) integrationMarkerResponse {
	return integrationMarkerResponse{
		Marker:                  marker,
		IntegrationSource:       "api_key",
		CreatedAgo:              humanizeRFC3339Duration(marker.CreatedAt, integrationNowFn()),
		EditableWithSameAPIKey:  true,
		RemovableWithSameAPIKey: true,
	}
}

func humanizeRFC3339Duration(timestamp string, now time.Time) string {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(timestamp))
	if err != nil {
		return ""
	}
	diff := now.Sub(parsed)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		minutes := int(diff / time.Minute)
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if diff < 24*time.Hour {
		hours := int(diff / time.Hour)
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(diff / (24 * time.Hour))
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

func writeIntegrationError(w http.ResponseWriter, statusCode int, code string, message string) {
	respondJSON(w, statusCode, integrationErrorResponse{
		Code:    strings.TrimSpace(code),
		Message: strings.TrimSpace(message),
	})
}

func isValidMarkerOutcomeStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case dbmodel.MarkerCreationOutcomeStatusSuccess, dbmodel.MarkerCreationOutcomeStatusFailed:
		return true
	default:
		return false
	}
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func recordStaticPreviewFailure(apiKey *dbmodel.APIKey, sourceIP string, reason string, username string) {
	if apiKey == nil {
		return
	}
	_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
		Operation: "integration.static_preview.generate.result",
		SourceIP:  strings.TrimSpace(sourceIP),
		Success:   false,
		Reason:    fmt.Sprintf("username=%s; reason=%s", strings.TrimSpace(username), strings.TrimSpace(reason)),
	})
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

	log.Printf("api-key request key_id=%d key_name=%s operation=%s scope=%s source_ip=%s", apiKey.ID, apiKey.Name, operationWithContext, requiredScope, sourceIP)
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
