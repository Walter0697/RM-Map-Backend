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
	"mapmarker/backend/utils"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

type createAPIKeyRequest struct {
	Name             string   `json:"name"`
	Testing          *bool    `json:"testing,omitempty"`
	Scopes           []string `json:"scopes"`
	RelationID       uint     `json:"relation_id"`
	ActorUserID      *uint    `json:"actor_user_id,omitempty"`
	ServiceAccountID *uint    `json:"service_account_id,omitempty"`
	ExpiresAt        *string  `json:"expires_at"`
}

type apiKeyResponse struct {
	ID                 uint       `json:"id"`
	Name               string     `json:"name"`
	Testing            bool       `json:"testing"`
	Prefix             string     `json:"prefix"`
	Scopes             []string   `json:"scopes"`
	Status             string     `json:"status"`
	RelationID         uint       `json:"relation_id"`
	ActorUserID        *uint      `json:"actor_user_id"`
	ServiceAccountID   *uint      `json:"service_account_id,omitempty"`
	ServiceAccountName *string    `json:"service_account_name,omitempty"`
	LastUsedAt         *time.Time `json:"last_used_at"`
	ExpiresAt          *time.Time `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
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

type apiKeyServiceAccountOptionResponse struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Role           string `json:"role"`
	RelationID     uint   `json:"relation_id"`
	Active         bool   `json:"active"`
	ActingUserID   *uint  `json:"acting_user_id,omitempty"`
	ActingUsername string `json:"acting_username,omitempty"`
}

type integrationCreateMarkerRequest struct {
	Label             string  `json:"label"`
	Testing           *bool   `json:"testing,omitempty"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	Address           string  `json:"address"`
	Type              string  `json:"type"`
	ImageLink         *string `json:"image_link"`
	Link              *string `json:"link"`
	SocialMediaLink   *string `json:"social_media_link"`
	Description       *string `json:"description"`
	Permanent         *bool   `json:"permanent"`
	NeedBooking       *bool   `json:"need_booking"`
	ToTime            *string `json:"to_time"`
	FromTime          *string `json:"from_time"`
	EstimateTime      *string `json:"estimate_time"`
	RestaurantID      *int    `json:"restaurant_id"`
	WebsiteProvider   *string `json:"website_provider"`
	WebsiteProviderID *string `json:"website_provider_id"`
	Price             *string `json:"price"`
}

type integrationUpdateMarkerRequest struct {
	Label             *string `json:"label"`
	Testing           *bool   `json:"testing,omitempty"`
	Address           *string `json:"address"`
	ImageLink         *string `json:"image_link"`
	NoImage           bool    `json:"no_image"`
	Link              *string `json:"link"`
	SocialMediaLink   *string `json:"social_media_link"`
	Type              *string `json:"type"`
	Description       *string `json:"description"`
	Permanent         *bool   `json:"permanent"`
	NeedBooking       *bool   `json:"need_booking"`
	ToTime            *string `json:"to_time"`
	FromTime          *string `json:"from_time"`
	EstimateTime      *string `json:"estimate_time"`
	RestaurantID      *int    `json:"restaurant_id"`
	WebsiteProvider   *string `json:"website_provider"`
	WebsiteProviderID *string `json:"website_provider_id"`
	RemoveRestaurant  *bool   `json:"remove_restaurant"`
	Price             *string `json:"price"`
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
	Label        string                                  `json:"label"`
	Testing      *bool                                   `json:"testing,omitempty"`
	Description  string                                  `json:"description"`
	SelectedTime string                                  `json:"selected_time"`
	MarkerID     int                                     `json:"marker_id"`
	RoutePreview *integrationScheduleRoutePreviewRequest `json:"route_preview,omitempty"`
}

type integrationUpdateScheduleRequest struct {
	Label        *string                                 `json:"label,omitempty"`
	Testing      *bool                                   `json:"testing,omitempty"`
	Description  *string                                 `json:"description,omitempty"`
	SelectedTime *string                                 `json:"selected_time,omitempty"`
	RoutePreview *integrationScheduleRoutePreviewRequest `json:"route_preview,omitempty"`
}

type integrationOverwriteScheduleByDateRequest struct {
	Date  string                             `json:"date"`
	Items []integrationCreateScheduleRequest `json:"items"`
}

type integrationOverwriteScheduleByDateResponse struct {
	Date         string                        `json:"date"`
	DeletedCount int64                         `json:"deleted_count"`
	CreatedCount int                           `json:"created_count"`
	Items        []integrationScheduleResponse `json:"items"`
}

type integrationCalendarGoogleSyncByDateRequest struct {
	Date string `json:"date"`
}

type integrationCalendarGoogleSyncByDateItem struct {
	ScheduleID uint   `json:"schedule_id"`
	Status     string `json:"status"`
	Action     string `json:"action,omitempty"`
	Error      string `json:"error,omitempty"`
}

type integrationCalendarGoogleSyncByDateResponse struct {
	Date     string                                    `json:"date"`
	Provider string                                    `json:"provider"`
	Total    int                                       `json:"total"`
	Synced   int                                       `json:"synced"`
	Failed   int                                       `json:"failed"`
	Items    []integrationCalendarGoogleSyncByDateItem `json:"items"`
}

type integrationScheduleRoutePreviewRequest struct {
	ImageRef       *string                             `json:"image_ref,omitempty"`
	DistanceMeters *int                                `json:"distance_meters,omitempty"`
	ETA            *integrationScheduleRouteETARequest `json:"eta,omitempty"`
}

type integrationScheduleRouteETARequest struct {
	WalkingSeconds       *int `json:"walking_seconds,omitempty"`
	BusSeconds           *int `json:"bus_seconds,omitempty"`
	PublicTransitSeconds *int `json:"public_transit_seconds,omitempty"`
}

type integrationScheduleRouteETAResponse struct {
	WalkingSeconds       *int `json:"walking_seconds,omitempty"`
	BusSeconds           *int `json:"bus_seconds,omitempty"`
	PublicTransitSeconds *int `json:"public_transit_seconds,omitempty"`
}

type integrationScheduleRoutePreviewResponse struct {
	ImageRef       *string                             `json:"image_ref,omitempty"`
	DistanceMeters *int                                `json:"distance_meters,omitempty"`
	ETA            integrationScheduleRouteETAResponse `json:"eta"`
}

type integrationScheduleResponse struct {
	model.Schedule
	Testing      bool                                     `json:"testing"`
	RoutePreview *integrationScheduleRoutePreviewResponse `json:"route_preview,omitempty"`
	Warnings     []string                                 `json:"warnings,omitempty"`
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

type integrationSettingsPinResponse struct {
	ID           uint     `json:"id"`
	Value        string   `json:"value,omitempty"`
	Label        string   `json:"label"`
	ImagePath    string   `json:"image_path"`
	DisplayPath  string   `json:"display_path"`
	TopLeftX     int      `json:"top_left_x,omitempty"`
	TopLeftY     int      `json:"top_left_y,omitempty"`
	BottomRightX int      `json:"bottom_right_x,omitempty"`
	BottomRightY int      `json:"bottom_right_y,omitempty"`
	GroupIDs     []uint   `json:"group_ids"`
	GroupNames   []string `json:"group_names"`
}

type updateOwnPreviewPinRequest struct {
	PinID *int `json:"pin_id"`
}

type integrationUpdateUserPreviewPinRequest struct {
	PinID *int `json:"pin_id"`
}

type integrationUpdateUserReminderTimeRequest struct {
	Time string `json:"time"`
}

type integrationUserPreviewPinResponse struct {
	Username     string `json:"username"`
	PinID        *uint  `json:"pin_id,omitempty"`
	PinLabel     string `json:"pin_label,omitempty"`
	PinImagePath string `json:"pin_image_path,omitempty"`
}

type integrationUserReminderTimeResponse struct {
	Username string `json:"username"`
	Time     string `json:"time"`
	Source   string `json:"source"`
}

type integrationDueReminderResponse struct {
	ScheduleID       uint    `json:"schedule_id"`
	ScheduleLabel    string  `json:"schedule_label"`
	ScheduleStatus   string  `json:"schedule_status"`
	ScheduleTime     string  `json:"schedule_time"`
	Marker           model.Marker `json:"marker"`
	MarkerID         uint    `json:"marker_id"`
	MarkerLabel      string  `json:"marker_label"`
	MarkerLatitude   float64 `json:"marker_latitude"`
	MarkerLongitude  float64 `json:"marker_longitude"`
	MarkerTimezone   string  `json:"marker_timezone"`
	LocalDate        string  `json:"local_date"`
	LocalNow         string  `json:"local_now"`
	ReminderTime     string  `json:"reminder_time"`
	ReminderAtLocal  string  `json:"reminder_at_local"`
	RelationID       uint    `json:"relation_id"`
	ReminderUsername string  `json:"username"`
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
	Testing                 bool                                   `json:"testing"`
	IntegrationSource       string                                 `json:"integration_source"`
	CreatedAgo              string                                 `json:"created_ago"`
	EditableWithSameAPIKey  bool                                   `json:"editable_with_same_api_key"`
	RemovableWithSameAPIKey bool                                   `json:"removable_with_same_api_key"`
	WebsiteIntegration      *integrationWebsiteIntegrationResponse `json:"website_integration,omitempty"`
}

type integrationWebsiteIntegrationResponse struct {
	Provider    string  `json:"provider"`
	ExternalID  string  `json:"external_id"`
	FetchStatus string  `json:"fetch_status"`
	Name        *string `json:"name,omitempty"`
	Rating      *string `json:"rating,omitempty"`
	URL         *string `json:"url,omitempty"`
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

type integrationNearbySearchQuery struct {
	Latitude     float64
	Longitude    float64
	RadiusMeters float64
	Limit        int
}

type integrationNearbyMarkerRow struct {
	dbmodel.Marker
	DistanceMeters float64 `json:"distance_meters" gorm:"column:distance_meters"`
}

type integrationNearbyMarkerResponse struct {
	integrationMarkerResponse
	DistanceMeters float64 `json:"distance_meters"`
}

const (
	integrationNearbyDefaultLimit      = 20
	integrationNearbyMaxLimit          = 50
	integrationNearbyMaxRadiusMeters   = 50000.0
	integrationNearbyEarthRadiusMeters = 6371000.0
)

var integrationGetUserPreviewPinFn = GetUserPreviewPinSelection
var integrationSetUserPreviewPinFn = SetUserPreviewPinSelection
var integrationGenerateStaticMapFn = GenerateStaticMapPreviewByUsername
var integrationGeocodeAddressFn = GeocodeStreetAddress
var integrationAuthenticateRequestFn = authenticateIntegrationRequest
var integrationAuthenticateNearbyRequestFn = authenticateIntegrationRequestJSON
var integrationCreateAuditLogFn = CreateAPIKeyAuditLog
var integrationCreateMarkerOutcomeLogFn = CreateMarkerCreationOutcomeLog
var integrationNowFn = time.Now
var integrationGetDueScheduleRemindersByUsernameFn = getDueScheduleRemindersByUsername
var integrationGetUserReminderTimeFn = GetUserReminderTime
var integrationSetUserReminderTimeFn = SetUserReminderTime
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
var integrationFindNearbyMarkersFn = findNearbyMarkersByDistance
var integrationExecuteManualCalendarSyncFn = executeManualCalendarSync
var integrationListRelationSchedulesByDateFn = func(relationID uint, dayStart time.Time, dayEnd time.Time, includeTesting bool) ([]dbmodel.Schedule, error) {
	query := database.Connection.Model(&dbmodel.Schedule{}).Where("relation_id = ?", relationID)
	query = query.Where("selected_date >= ? AND selected_date < ?", dayStart.Format(time.RFC3339), dayEnd.Format(time.RFC3339))
	if !includeTesting {
		query = query.Where("testing = ?", false)
	}
	items := make([]dbmodel.Schedule, 0)
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
var integrationResolveCalendarSyncUserIDFn = func(apiKey *dbmodel.APIKey) (uint, error) {
	if apiKey == nil {
		return 0, fmt.Errorf("api key context missing")
	}
	if apiKey.ActorUser.ID != 0 {
		return apiKey.ActorUser.ID, nil
	}
	if apiKey.ActorUserID != nil && *apiKey.ActorUserID != 0 {
		return *apiKey.ActorUserID, nil
	}
	if apiKey.Relation.UserOneUID != 0 {
		return apiKey.Relation.UserOneUID, nil
	}
	if apiKey.Relation.UserTwoUID != 0 {
		return apiKey.Relation.UserTwoUID, nil
	}
	return 0, fmt.Errorf("unable to resolve calendar sync actor user")
}
var integrationBeginScheduleOverwriteTxFn = func() (*gorm.DB, error) {
	tx := database.Connection.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return tx, nil
}
var integrationListSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) ([]dbmodel.Schedule, error) {
	items := make([]dbmodel.Schedule, 0)
	err := tx.Preload("SelectedMarker").
		Where("relation_id = ?", relationID).
		Where("selected_date >= ? AND selected_date < ?", start.Format(time.RFC3339), end.Format(time.RFC3339)).
		Find(&items).Error
	return items, err
}
var integrationDeleteSchedulesByDateFn = func(tx *gorm.DB, relationID uint, start time.Time, end time.Time) (int64, error) {
	result := tx.Where("relation_id = ?", relationID).
		Where("selected_date >= ? AND selected_date < ?", start.Format(time.RFC3339), end.Format(time.RFC3339)).
		Delete(&dbmodel.Schedule{})
	return result.RowsAffected, result.Error
}
var integrationGetScheduleMarkerByIDFn = func(tx *gorm.DB, markerID uint) (*dbmodel.Marker, error) {
	marker := &dbmodel.Marker{}
	marker.ID = markerID
	if err := marker.GetById(tx); err != nil {
		return nil, err
	}
	return marker, nil
}
var integrationCreateScheduleFn = CreateSchedule
var integrationUpdateScheduleModelFn = func(tx *gorm.DB, schedule *dbmodel.Schedule) error {
	return schedule.Update(tx)
}
var integrationResetMarkerStatusForOverwriteFn = func(tx *gorm.DB, marker *dbmodel.Marker, actor dbmodel.User) error {
	if marker == nil {
		return nil
	}
	marker.Status = ""
	marker.UpdatedBy = &actor
	return marker.Update(tx)
}
var integrationCommitScheduleOverwriteTxFn = func(tx *gorm.DB) error {
	return tx.Commit().Error
}
var integrationRollbackScheduleOverwriteTxFn = func(tx *gorm.DB) {
	if tx == nil {
		return
	}
	tx.Rollback()
}

func normalizeSettingsPinLabel(value string, rawLabel *string) string {
	if rawLabel == nil {
		return value
	}
	trimmed := strings.TrimSpace(*rawLabel)
	if trimmed == "" {
		return value
	}
	return trimmed
}

func canAccessTestingEntities(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "admin")
}

func resolveRequestedTestingFlag(requested *bool, canAccess bool) (bool, error) {
	if requested == nil {
		return false, nil
	}
	if *requested && !canAccess {
		return false, fmt.Errorf("permission denied")
	}
	return *requested, nil
}

func resolveCreateTestingFlagForAPIKey(apiKey *dbmodel.APIKey, requested *bool, canAccess bool) (bool, error) {
	// Testing API keys always create testing entities, regardless of payload.
	if apiKey != nil && apiKey.Testing {
		return true, nil
	}
	return resolveRequestedTestingFlag(requested, canAccess)
}

func toIntegrationSettingsPinResponse(pin dbmodel.Pin) integrationSettingsPinResponse {
	groupIDs := make([]uint, 0, len(pin.Groups))
	groupNames := make([]string, 0, len(pin.Groups))
	for _, group := range pin.Groups {
		groupIDs = append(groupIDs, group.ID)
		groupNames = append(groupNames, group.Name)
	}

	return integrationSettingsPinResponse{
		ID:           pin.ID,
		Value:        pin.Label,
		Label:        normalizeSettingsPinLabel(pin.Label, pin.SettingsLabel),
		ImagePath:    pin.ImagePath,
		DisplayPath:  pin.DisplayPath,
		TopLeftX:     pin.TopLeftX,
		TopLeftY:     pin.TopLeftY,
		BottomRightX: pin.BottomRightX,
		BottomRightY: pin.BottomRightY,
		GroupIDs:     groupIDs,
		GroupNames:   groupNames,
	}
}

var integrationResolveRestaurantByProviderFn = GetOrCreateRestaurantByProvider

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
		Name:             request.Name,
		Testing:          request.Testing != nil && *request.Testing,
		Scopes:           request.Scopes,
		RelationID:       request.RelationID,
		ActorUserID:      request.ActorUserID,
		ServiceAccountID: request.ServiceAccountID,
		ExpiresAt:        expiresAt,
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
	testingFilter := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("testing")))
	if testingFilter == "true" || testingFilter == "false" {
		filtered := make([]dbmodel.APIKey, 0, len(keys))
		for _, item := range keys {
			if testingFilter == "true" && item.Testing {
				filtered = append(filtered, item)
				continue
			}
			if testingFilter == "false" && !item.Testing {
				filtered = append(filtered, item)
			}
		}
		keys = filtered
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

	users, relations, serviceAccounts, err := ListAPIKeyOptions()
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
	serviceAccountResponse := make([]apiKeyServiceAccountOptionResponse, 0, len(serviceAccounts))
	for _, item := range serviceAccounts {
		serviceAccountResponse = append(serviceAccountResponse, apiKeyServiceAccountOptionResponse{
			ID:             item.ID,
			Name:           item.Name,
			Description:    item.Description,
			Role:           item.Role,
			RelationID:     item.RelationID,
			Active:         item.Active,
			ActingUserID:   item.ActingUserID,
			ActingUsername: item.ActingUsername,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"users":            userResponse,
		"relations":        relationResponse,
		"service_accounts": serviceAccountResponse,
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
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "updated_at", []string{"type", "status", "country", "country_code", "label", "search", "west", "south", "east", "north", "zoom", "testing"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.markers.list", constant.APIKeyScopeMarkersRead, queryContextString(queryOption))
	if !ok {
		return
	}

	current := time.Now().AddDate(0, 0, -1)
	query := database.Connection.Model(&dbmodel.Marker{}).Preload("RestaurantInfo")
	query = query.Where("relation_id = ?", apiKey.Relation.ID)
	query = query.Where("status != ?", constant.Arrived)
	query = query.Where("to_time IS NULL OR (to_time IS NOT NULL AND to_time >= ?)", current.Format(time.RFC3339))
	// Enforce hidden-marker filtering via marker type visibility.
	query = query.Where(
		`type IN (SELECT value FROM marker_types WHERE hidden = FALSE)`,
	)

	canAccessTesting := canAccessTestingEntities(APIKeyActorRole(apiKey))
	if value, ok := queryOption.Filters["testing"]; ok {
		requestTesting := strings.EqualFold(strings.TrimSpace(value), "true")
		if requestTesting && !canAccessTesting {
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}
		query = query.Where("testing = ?", requestTesting)
	} else if !canAccessTesting {
		query = query.Where("testing = ?", false)
	}

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
	for index, marker := range response {
		decorated = append(decorated, buildIntegrationMarkerResponse(marker, markers[index].Testing))
	}
	nextCursor := ""
	if len(markers) == queryOption.Limit {
		nextCursor = strconv.FormatUint(uint64(markers[len(markers)-1].ID), 10)
	}
	respondJSON(w, http.StatusOK, integrationListResponse(decorated, total, queryOption, nextCursor))
}

func IntegrationNearbySearchMarkersHandler(w http.ResponseWriter, r *http.Request) {
	searchQuery, err := parseIntegrationNearbySearchQuery(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_nearby_search_input", err.Error())
		return
	}

	queryContext := fmt.Sprintf("lat=%.6f;lon=%.6f;radius=%.2f;limit=%d", searchQuery.Latitude, searchQuery.Longitude, searchQuery.RadiusMeters, searchQuery.Limit)
	apiKey, ok := integrationAuthenticateNearbyRequestFn(w, r, "integration.markers.nearby_search", constant.APIKeyScopeMarkersRead, queryContext)
	if !ok {
		return
	}

	rows, err := integrationFindNearbyMarkersFn(apiKey.Relation, searchQuery, canAccessTestingEntities(APIKeyActorRole(apiKey)))
	if err != nil {
		writeIntegrationError(w, http.StatusInternalServerError, "nearby_search_failed", "failed to retrieve nearby markers")
		return
	}

	items := make([]integrationNearbyMarkerResponse, 0, len(rows))
	for _, row := range rows {
		decorated := buildIntegrationMarkerResponse(helper.ConvertMarker(row.Marker), row.Marker.Testing)
		items = append(items, integrationNearbyMarkerResponse{
			integrationMarkerResponse: decorated,
			DistanceMeters:            row.DistanceMeters,
		})
	}

	responseQuery := integrationListQuery{
		Limit:   searchQuery.Limit,
		Offset:  0,
		Cursor:  0,
		SortBy:  "distance_meters",
		Order:   "asc",
		Filters: map[string]string{},
	}
	respondJSON(w, http.StatusOK, integrationListResponse(items, int64(len(items)), responseQuery, ""))
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

	testing, err := resolveCreateTestingFlagForAPIKey(apiKey, request.Testing, canAccessTestingEntities(APIKeyActorRole(apiKey)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
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
		SocialMediaLink: request.SocialMediaLink,
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
	provider := strings.TrimSpace(optionalStringValue(request.WebsiteProvider))
	providerID := strings.TrimSpace(optionalStringValue(request.WebsiteProviderID))
	if request.RestaurantID != nil && (provider != "" || providerID != "") {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_website_integration", "use either restaurant_id or website_provider fields")
		return
	}
	if provider != "" || providerID != "" {
		resolvedRestaurant, resolveErr := integrationResolveRestaurantByProviderFn(provider, providerID)
		if resolveErr != nil {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_website_integration", resolveErr.Error())
			return
		}
		restaurantPtr = resolvedRestaurant
	} else if request.RestaurantID != nil {
		restaurant.ID = uint(*request.RestaurantID)
		if err := restaurant.GetById(database.Connection); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		restaurantPtr = &restaurant
	}

	marker, err := CreateMarker(input, restaurantPtr, apiKey.ActorUser, apiKey.Relation, testing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, buildIntegrationMarkerResponse(helper.ConvertMarker(*marker), marker.Testing))
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
		SocialMediaLink:  request.SocialMediaLink,
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
	provider := strings.TrimSpace(optionalStringValue(request.WebsiteProvider))
	providerID := strings.TrimSpace(optionalStringValue(request.WebsiteProviderID))
	if request.RestaurantID != nil && (provider != "" || providerID != "") {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_website_integration", "use either restaurant_id or website_provider fields")
		return
	}
	if request.RemoveRestaurant != nil && *request.RemoveRestaurant && (provider != "" || providerID != "") {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_website_integration", "remove_restaurant cannot be combined with website_provider fields")
		return
	}
	if provider != "" || providerID != "" {
		resolvedRestaurant, resolveErr := integrationResolveRestaurantByProviderFn(provider, providerID)
		if resolveErr != nil {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_website_integration", resolveErr.Error())
			return
		}
		restaurantPtr = resolvedRestaurant
	} else if request.RestaurantID != nil {
		restaurant.ID = uint(*request.RestaurantID)
		if err := restaurant.GetById(database.Connection); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		restaurantPtr = &restaurant
	}

	canModifyTesting := canAccessTestingEntities(APIKeyActorRole(apiKey))
	marker, err := EditMarker(input, restaurantPtr, apiKey.Relation, apiKey.ActorUser, request.Testing, canModifyTesting)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, buildIntegrationMarkerResponse(helper.ConvertMarker(*marker), marker.Testing))
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
	if marker.Testing && !canAccessTestingEntities(APIKeyActorRole(apiKey)) {
		http.Error(w, "permission denied", http.StatusForbidden)
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
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "selected_date", []string{"status", "marker_id", "label", "search", "from", "to", "time", "testing"})
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
	canAccessTesting := canAccessTestingEntities(APIKeyActorRole(apiKey))
	if value, ok := queryOption.Filters["testing"]; ok {
		requestTesting := strings.EqualFold(strings.TrimSpace(value), "true")
		if requestTesting && !canAccessTesting {
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}
		query = query.Where("testing = ?", requestTesting)
	} else if !canAccessTesting {
		query = query.Where("testing = ?", false)
	}
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

	response := make([]integrationScheduleResponse, 0, len(schedules))
	for _, item := range schedules {
		response = append(response, buildIntegrationScheduleResponse(item, nil))
	}
	nextCursor := ""
	if len(schedules) == queryOption.Limit {
		nextCursor = strconv.FormatUint(uint64(schedules[len(schedules)-1].ID), 10)
	}
	payload := integrationListResponse(response, total, queryOption, nextCursor)
	payload["transition_analysis"] = BuildScheduleTransitionAnalysis(BuildScheduleTravelPointsFromSchedules(schedules))
	respondJSON(w, http.StatusOK, payload)
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

	canModifyTesting := canAccessTestingEntities(APIKeyActorRole(apiKey))
	requestedTesting, err := resolveCreateTestingFlagForAPIKey(apiKey, request.Testing, canModifyTesting)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
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
	if marker.Testing && !canModifyTesting {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	input := model.NewSchedule{
		Label:        request.Label,
		Description:  request.Description,
		SelectedTime: request.SelectedTime,
		MarkerID:     request.MarkerID,
	}

	tx := database.Connection.Begin()
	scheduleTesting := requestedTesting || marker.Testing
	schedule, err := CreateSchedule(tx, input, marker, apiKey.ActorUser, apiKey.Relation, scheduleTesting)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	warnings := []string{}
	if request.RoutePreview != nil {
		applyWarnings, applyErr := applyRoutePreviewToSchedule(schedule, request.RoutePreview)
		if applyErr != nil {
			warnings = append(warnings, applyErr.Error())
			schedule.RoutePreviewWarning = trimmedStringPtr(strings.TrimSpace(applyErr.Error()))
		}
		warnings = append(warnings, applyWarnings...)
	}

	if err := schedule.Update(tx); err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, buildIntegrationScheduleResponse(*schedule, warnings))
}

func IntegrationUpdateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.schedules.update", constant.APIKeyScopeSchedulesWrite, "")
	if !ok {
		return
	}

	scheduleID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || scheduleID <= 0 {
		http.Error(w, "invalid schedule id", http.StatusBadRequest)
		return
	}

	requestPayload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request := integrationUpdateScheduleRequest{}
	if err := decodeStrictJSONPayload(requestPayload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	schedule := dbmodel.Schedule{}
	schedule.ID = uint(scheduleID)
	if err := schedule.GetById(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if schedule.RelationId != apiKey.Relation.ID {
		http.Error(w, "api key cannot access schedule outside assigned relation", http.StatusForbidden)
		return
	}
	canModifyTesting := canAccessTestingEntities(APIKeyActorRole(apiKey))
	if schedule.Testing && !canModifyTesting {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	if request.Label != nil {
		schedule.Label = strings.TrimSpace(*request.Label)
	}
	if request.Description != nil {
		schedule.Description = strings.TrimSpace(*request.Description)
	}
	if request.SelectedTime != nil {
		parsedTime, selectedLocalDate, selectedLocalTime, parseErr := parseScheduleSelectedTime(strings.TrimSpace(*request.SelectedTime), schedule.SelectedMarker)
		if parseErr != nil {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_selected_time", "selected_time must be RFC3339 or 2006-01-02 15:04:05")
			return
		}
		schedule.SelectedDate = parsedTime
		schedule.SelectedLocalDate = selectedLocalDate
		schedule.SelectedLocalTime = selectedLocalTime
	}

	warnings := []string{}
	if request.RoutePreview != nil {
		applyWarnings, applyErr := applyRoutePreviewToSchedule(&schedule, request.RoutePreview)
		if applyErr != nil {
			warnings = append(warnings, applyErr.Error())
			schedule.RoutePreviewWarning = trimmedStringPtr(strings.TrimSpace(applyErr.Error()))
		}
		warnings = append(warnings, applyWarnings...)
	}

	schedule.UpdatedBy = &apiKey.ActorUser
	if request.Testing != nil {
		if !canModifyTesting {
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}
		schedule.Testing = *request.Testing
	}
	if err := schedule.Update(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, buildIntegrationScheduleResponse(schedule, warnings))
}

func IntegrationOverwriteSchedulesByDateHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.schedules.overwrite_by_date", constant.APIKeyScopeSchedulesWrite, "")
	if !ok {
		return
	}
	canModifyTesting := canAccessTestingEntities(APIKeyActorRole(apiKey))

	requestPayload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request := integrationOverwriteScheduleByDateRequest{}
	if err := decodeStrictJSONPayload(requestPayload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request.Date = strings.TrimSpace(request.Date)
	if request.Date == "" {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_target_date", "date is required and must be YYYY-MM-DD")
		return
	}
	dayStart, err := time.Parse(utils.DayOnlyTime, request.Date)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_target_date", "date is required and must be YYYY-MM-DD")
		return
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	for index := range request.Items {
		item := &request.Items[index]
		item.Label = strings.TrimSpace(item.Label)
		item.Description = strings.TrimSpace(item.Description)
		if item.Label == "" {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_schedule_item", "label is required for each schedule item")
			return
		}
		if item.MarkerID <= 0 {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_schedule_item", "marker_id must be a positive integer for each schedule item")
			return
		}
	}

	tx, err := integrationBeginScheduleOverwriteTxFn()
	if err != nil {
		writeIntegrationError(w, http.StatusInternalServerError, "overwrite_failed", "failed to begin overwrite transaction")
		return
	}
	defer integrationRollbackScheduleOverwriteTxFn(tx)

	existingSchedules, err := integrationListSchedulesByDateFn(tx, apiKey.Relation.ID, dayStart, dayEnd)
	if err != nil {
		writeIntegrationError(w, http.StatusInternalServerError, "overwrite_failed", "failed to read existing schedules for target date")
		return
	}

	resetMarkerIDs := map[uint]struct{}{}
	for _, existing := range existingSchedules {
		if existing.SelectedMarker == nil || existing.SelectedMarker.ID == 0 {
			continue
		}
		if _, exists := resetMarkerIDs[existing.SelectedMarker.ID]; exists {
			continue
		}
		if err := integrationResetMarkerStatusForOverwriteFn(tx, existing.SelectedMarker, apiKey.ActorUser); err != nil {
			writeIntegrationError(w, http.StatusInternalServerError, "overwrite_failed", "failed to reset marker status before overwrite")
			return
		}
		resetMarkerIDs[existing.SelectedMarker.ID] = struct{}{}
	}

	deletedCount, err := integrationDeleteSchedulesByDateFn(tx, apiKey.Relation.ID, dayStart, dayEnd)
	if err != nil {
		writeIntegrationError(w, http.StatusInternalServerError, "overwrite_failed", "failed to remove existing schedules for target date")
		return
	}

	createdItems := make([]integrationScheduleResponse, 0, len(request.Items))
	for _, item := range request.Items {
		marker, err := integrationGetScheduleMarkerByIDFn(tx, uint(item.MarkerID))
		if err != nil {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_schedule_item", "marker_id references a non-existent marker")
			return
		}
		if marker.RelationId != apiKey.Relation.ID {
			writeIntegrationError(w, http.StatusForbidden, "api_key_scope_denied", "api key cannot access marker outside assigned relation")
			return
		}
		if marker.Testing && !canModifyTesting {
			writeIntegrationError(w, http.StatusForbidden, "permission_denied", "permission denied")
			return
		}
		normalizedSelectedTime := strings.TrimSpace(item.SelectedTime)
		if _, parseClockErr := time.Parse("15:04", normalizedSelectedTime); parseClockErr == nil {
			normalizedSelectedTime = fmt.Sprintf("%s %s:00", dayStart.Format(utils.DayOnlyTime), normalizedSelectedTime)
		}
		_, selectedLocalDate, _, parseErr := parseScheduleSelectedTime(normalizedSelectedTime, marker)
		if parseErr != nil {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_schedule_item", "selected_time must be HH:MM or 2006-01-02 15:04:05")
			return
		}
		if selectedLocalDate != dayStart.Format(utils.DayOnlyTime) {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_schedule_item", "selected_time must be on the target date")
			return
		}
		requestedTesting, err := resolveCreateTestingFlagForAPIKey(apiKey, item.Testing, canModifyTesting)
		if err != nil {
			writeIntegrationError(w, http.StatusForbidden, "permission_denied", err.Error())
			return
		}

		label := item.Label
		if apiKey.Testing {
			label = "testing schedule"
		}
		scheduleTesting := requestedTesting || marker.Testing

		schedule, err := integrationCreateScheduleFn(tx, model.NewSchedule{
			Label:        label,
			Description:  item.Description,
			SelectedTime: normalizedSelectedTime,
			MarkerID:     item.MarkerID,
		}, *marker, apiKey.ActorUser, apiKey.Relation, scheduleTesting)
		if err != nil {
			writeIntegrationError(w, http.StatusBadRequest, "invalid_schedule_item", err.Error())
			return
		}

		warnings := []string{}
		if item.RoutePreview != nil {
			applyWarnings, applyErr := applyRoutePreviewToSchedule(schedule, item.RoutePreview)
			if applyErr != nil {
				warnings = append(warnings, applyErr.Error())
				schedule.RoutePreviewWarning = trimmedStringPtr(strings.TrimSpace(applyErr.Error()))
			}
			warnings = append(warnings, applyWarnings...)
		}
		if err := integrationUpdateScheduleModelFn(tx, schedule); err != nil {
			writeIntegrationError(w, http.StatusInternalServerError, "overwrite_failed", "failed to persist overwritten schedules")
			return
		}

		createdItems = append(createdItems, buildIntegrationScheduleResponse(*schedule, warnings))
	}

	if err := integrationCommitScheduleOverwriteTxFn(tx); err != nil {
		writeIntegrationError(w, http.StatusInternalServerError, "overwrite_failed", "failed to commit overwrite transaction")
		return
	}

	respondJSON(w, http.StatusOK, integrationOverwriteScheduleByDateResponse{
		Date:         dayStart.Format(utils.DayOnlyTime),
		DeletedCount: deletedCount,
		CreatedCount: len(createdItems),
		Items:        createdItems,
	})
}

func IntegrationCalendarGoogleSyncByDateHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.calendar.google.sync_by_date", constant.APIKeyScopeCalendarSync, "")
	if !ok {
		return
	}

	requestPayload, err := readJSONBody(r)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}
	request := integrationCalendarGoogleSyncByDateRequest{}
	if err := decodeStrictJSONPayload(requestPayload, &request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	request.Date = strings.TrimSpace(request.Date)
	if request.Date == "" {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_target_date", "date is required and must be YYYY-MM-DD")
		return
	}
	dayStart, err := time.Parse(utils.DayOnlyTime, request.Date)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_target_date", "date is required and must be YYYY-MM-DD")
		return
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	syncUserID, err := integrationResolveCalendarSyncUserIDFn(apiKey)
	if err != nil {
		writeIntegrationError(w, http.StatusForbidden, "calendar_sync_actor_unavailable", "unable to resolve calendar sync actor user")
		return
	}

	includeTesting := canAccessTestingEntities(APIKeyActorRole(apiKey))
	schedules, err := integrationListRelationSchedulesByDateFn(apiKey.Relation.ID, dayStart, dayEnd, includeTesting)
	if err != nil {
		writeIntegrationError(w, http.StatusInternalServerError, "calendar_sync_query_failed", "failed to load schedules for date")
		return
	}

	items := make([]integrationCalendarGoogleSyncByDateItem, 0, len(schedules))
	synced := 0
	failed := 0
	for _, schedule := range schedules {
		result, syncErr := integrationExecuteManualCalendarSyncFn(r.Context(), syncUserID, schedule.ID, CalendarProviderGoogle, "")
		if syncErr != nil {
			failed++
			items = append(items, integrationCalendarGoogleSyncByDateItem{
				ScheduleID: schedule.ID,
				Status:     "failed",
				Error:      syncErr.Error(),
			})
			continue
		}
		synced++
		items = append(items, integrationCalendarGoogleSyncByDateItem{
			ScheduleID: schedule.ID,
			Status:     strings.TrimSpace(result.SyncStatus),
			Action:     strings.TrimSpace(result.Action),
		})
	}

	respondJSON(w, http.StatusOK, integrationCalendarGoogleSyncByDateResponse{
		Date:     dayStart.Format(utils.DayOnlyTime),
		Provider: CalendarProviderGoogle,
		Total:    len(schedules),
		Synced:   synced,
		Failed:   failed,
		Items:    items,
	})
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
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "label", []string{"label", "group_id", "group_name"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, ok := authenticateIntegrationRequest(w, r, "integration.settings.pins.list", constant.APIKeyScopeSettingsRead, queryContextString(queryOption))
	if !ok {
		return
	}

	query := database.Connection.Model(&dbmodel.Pin{}).Preload("Groups")
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("label ILIKE ?", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["group_id"]; ok {
		groupID, convErr := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if convErr != nil || groupID == 0 {
			http.Error(w, "invalid group_id", http.StatusBadRequest)
			return
		}
		query = query.Joins("JOIN pin_group_assignments pga ON pga.pin_id = pins.id").Where("pga.pin_group_id = ?", uint(groupID))
	}
	if value, ok := queryOption.Filters["group_name"]; ok {
		query = query.
			Joins("JOIN pin_group_assignments pga_filter ON pga_filter.pin_id = pins.id").
			Joins("JOIN pin_groups pg_filter ON pg_filter.id = pga_filter.pin_group_id").
			Where("pg_filter.name ILIKE ?", "%"+strings.TrimSpace(value)+"%")
	}

	query = query.Distinct("pins.id")

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

	responseItems := make([]integrationSettingsPinResponse, 0, len(items))
	for _, item := range items {
		groupIDs := make([]uint, 0, len(item.Groups))
		groupNames := make([]string, 0, len(item.Groups))
		for _, group := range item.Groups {
			groupIDs = append(groupIDs, group.ID)
			groupNames = append(groupNames, group.Name)
		}
		responseItems = append(responseItems, integrationSettingsPinResponse{
			ID:          item.ID,
			Label:       item.Label,
			ImagePath:   item.ImagePath,
			DisplayPath: item.DisplayPath,
			GroupIDs:    groupIDs,
			GroupNames:  groupNames,
		})
	}

	respondJSON(w, http.StatusOK, integrationListResponse(responseItems, total, queryOption, ""))
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
			response.PinImagePath = preference.PreviewPin.ImagePath
		} else if label, labelErr := resolvePreviewPinLabelByID(*preference.PreviewPinID); labelErr == nil {
			response.PinLabel = label
		}
		if response.PinImagePath == "" {
			if imagePath, imagePathErr := resolvePreviewPinImagePathByID(*preference.PreviewPinID); imagePathErr == nil {
				response.PinImagePath = imagePath
			}
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
		Username:     user.Username,
		PinID:        preference.PreviewPinID,
		PinLabel:     pin.Label,
		PinImagePath: pin.ImagePath,
	})
}

func SettingsGetReminderTimeHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	_, reminderTime, source, err := integrationGetUserReminderTimeFn(user.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, integrationUserReminderTimeResponse{
		Username: user.Username,
		Time:     reminderTime,
		Source:   source,
	})
}

func SettingsUpdateReminderTimeHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	request := integrationUpdateUserReminderTimeRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, reminderTime, err := integrationSetUserReminderTimeFn(user.Username, request.Time, user)
	if err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_reminder_time", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, integrationUserReminderTimeResponse{
		Username: user.Username,
		Time:     reminderTime,
		Source:   "user",
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
			response.PinImagePath = preference.PreviewPin.ImagePath
		} else if label, labelErr := resolvePreviewPinLabelByID(*preference.PreviewPinID); labelErr == nil {
			response.PinLabel = label
		}
		if response.PinImagePath == "" {
			if imagePath, imagePathErr := resolvePreviewPinImagePathByID(*preference.PreviewPinID); imagePathErr == nil {
				response.PinImagePath = imagePath
			}
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
		Username:     username,
		PinID:        preference.PreviewPinID,
		PinLabel:     pin.Label,
		PinImagePath: pin.ImagePath,
	})
}

func IntegrationGetUserReminderTimeHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := integrationAuthenticateRequestFn(w, r, "integration.settings.reminder_time.get", constant.APIKeyScopeSettingsRead, "")
	if !ok {
		return
	}

	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	user, reminderTime, source, err := integrationGetUserReminderTimeFn(username)
	if err != nil {
		if err == ErrUnknownUsername {
			writeIntegrationError(w, http.StatusNotFound, "unknown_username", "username does not exist")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, integrationUserReminderTimeResponse{
		Username: user.Username,
		Time:     reminderTime,
		Source:   source,
	})
}

func IntegrationUpdateUserReminderTimeHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateRequestFn(w, r, "integration.settings.reminder_time.update", constant.APIKeyScopeSettingsWrite, "")
	if !ok {
		return
	}

	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	request := integrationUpdateUserReminderTimeRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, reminderTime, err := integrationSetUserReminderTimeFn(username, request.Time, &apiKey.ActorUser)
	if err != nil {
		switch err {
		case ErrUnknownUsername:
			writeIntegrationError(w, http.StatusNotFound, "unknown_username", "username does not exist")
		default:
			writeIntegrationError(w, http.StatusBadRequest, "invalid_reminder_time", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, integrationUserReminderTimeResponse{
		Username: username,
		Time:     reminderTime,
		Source:   "user",
	})
}

func IntegrationListDueScheduleRemindersHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateNearbyRequestFn(w, r, "integration.reminders.due", constant.APIKeyScopeSchedulesRead, "")
	if !ok {
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_username", "username is required")
		return
	}

	items, todayScheduleItemsCount, reminderWindowIsActive, willRemindAt, markerLocationCurrentTime, markerLocationTimezone, err := integrationGetDueScheduleRemindersByUsernameFn(apiKey.Relation, username)
	if err != nil {
		switch err {
		case ErrUnknownUsername:
			writeIntegrationError(w, http.StatusNotFound, "unknown_username", "username does not exist")
		case ErrUserNotInAPIKeyRelation:
			writeIntegrationError(w, http.StatusForbidden, "username_not_in_relation", "username does not belong to api key relation")
		default:
			writeIntegrationError(w, http.StatusInternalServerError, "reminders_due_failed", "failed to retrieve due reminders")
		}
		return
	}

	responseItems := make([]integrationDueReminderResponse, 0, len(items))
	for _, item := range items {
		marker := item.Schedule.SelectedMarker
		if marker == nil {
			continue
		}
		responseItems = append(responseItems, integrationDueReminderResponse{
			ScheduleID:       item.Schedule.ID,
			ScheduleLabel:    item.Schedule.Label,
			ScheduleStatus:   item.Schedule.Status,
			ScheduleTime:     helper.ConvertSchedule(item.Schedule).SelectedDate,
			Marker:           helper.ConvertMarker(*marker),
			MarkerID:         marker.ID,
			MarkerLabel:      marker.Label,
			MarkerLatitude:   marker.Latitude,
			MarkerLongitude:  marker.Longitude,
			MarkerTimezone:   item.MarkerTimezone,
			LocalDate:        item.LocalDate,
			LocalNow:         item.LocalNow.Format(time.RFC3339),
			ReminderTime:     item.ReminderTime,
			ReminderAtLocal:  item.ReminderAt.Format(time.RFC3339),
			RelationID:       item.Schedule.RelationId,
			ReminderUsername: item.User.Username,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"username":                username,
		"items":                   responseItems,
		"total":                   len(responseItems),
		"reminder_time":           reminderWindowIsActive,
		"will_remind_at":          willRemindAt,
		"today_schedule_items_count": todayScheduleItemsCount,
		"marker_location_timezone": markerLocationTimezone,
		"marker_location_current_time": markerLocationCurrentTime,
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
	log.Printf(
		"integration geocode request key_id=%d key_name=%s source_ip=%s street_number=%q street_name=%q country=%q",
		apiKey.ID,
		apiKey.Name,
		requestSourceIP(r),
		request.StreetNumber,
		request.StreetName,
		request.Country,
	)

	lat, lon, geocodeErr := integrationGeocodeAddressFn(request.StreetNumber, request.StreetName, request.Country)
	if geocodeErr != nil {
		log.Printf(
			"integration geocode failure key_id=%d key_name=%s source_ip=%s error=%v",
			apiKey.ID,
			apiKey.Name,
			requestSourceIP(r),
			geocodeErr,
		)
		_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
			Operation: "integration.static_preview.geocode.result",
			SourceIP:  requestSourceIP(r),
			Success:   false,
			Reason:    geocodeErr.Error(),
		})
		writeIntegrationError(w, http.StatusBadGateway, "geocode_dependency_failure", "failed to geocode address to coordinates")
		return
	}
	log.Printf(
		"integration geocode result key_id=%d key_name=%s source_ip=%s lat=%f lon=%f",
		apiKey.ID,
		apiKey.Name,
		requestSourceIP(r),
		lat,
		lon,
	)

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

func buildIntegrationMarkerResponse(marker model.Marker, testing bool) integrationMarkerResponse {
	return integrationMarkerResponse{
		Marker:                  marker,
		Testing:                 testing,
		IntegrationSource:       "api_key",
		CreatedAgo:              humanizeRFC3339Duration(marker.CreatedAt, integrationNowFn()),
		EditableWithSameAPIKey:  true,
		RemovableWithSameAPIKey: true,
		WebsiteIntegration:      buildIntegrationWebsitePayload(marker),
	}
}

func buildIntegrationWebsitePayload(marker model.Marker) *integrationWebsiteIntegrationResponse {
	if marker.Restaurant == nil {
		return &integrationWebsiteIntegrationResponse{FetchStatus: "no_integration"}
	}

	provider := NormalizeMarkerWebsiteProviderID(marker.Restaurant.Source)
	externalID := strings.TrimSpace(marker.Restaurant.SourceID)
	if provider == "" && externalID != "" {
		// Backward-compat mapping for legacy OpenRice records with missing source.
		provider = constant.Openrice
	}

	item := &integrationWebsiteIntegrationResponse{
		Provider:    provider,
		ExternalID:  externalID,
		FetchStatus: "cached",
	}

	name := strings.TrimSpace(marker.Restaurant.Name)
	if name != "" {
		item.Name = &name
	}
	rating := normalizeIntegrationRestaurantRating(marker.Restaurant.Rating)
	if rating != "" {
		item.Rating = &rating
	}
	websiteURL := strings.TrimSpace(optionalStringValue(marker.Restaurant.Website))
	if websiteURL != "" {
		item.URL = &websiteURL
	}

	return item
}

func normalizeIntegrationRestaurantRating(raw *string) string {
	value := strings.TrimSpace(optionalStringValue(raw))
	if value == "" {
		return ""
	}
	var openriceRating struct {
		Like    string `json:"like"`
		Average string `json:"average"`
		Dislike string `json:"dislike"`
	}
	if err := json.Unmarshal([]byte(value), &openriceRating); err == nil {
		parts := make([]string, 0, 3)
		if strings.TrimSpace(openriceRating.Like) != "" {
			parts = append(parts, "like "+strings.TrimSpace(openriceRating.Like))
		}
		if strings.TrimSpace(openriceRating.Average) != "" {
			parts = append(parts, "average "+strings.TrimSpace(openriceRating.Average))
		}
		if strings.TrimSpace(openriceRating.Dislike) != "" {
			parts = append(parts, "dislike "+strings.TrimSpace(openriceRating.Dislike))
		}
		return strings.Join(parts, " | ")
	}
	return value
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

func buildIntegrationScheduleResponse(schedule dbmodel.Schedule, warnings []string) integrationScheduleResponse {
	modelSchedule := helper.ConvertSchedule(schedule)
	response := integrationScheduleResponse{
		Schedule: modelSchedule,
		Testing:  schedule.Testing,
	}
	if len(warnings) > 0 {
		response.Warnings = warnings
	}

	if schedule.RouteImageRef != nil || schedule.RouteDistanceMeters != nil || schedule.RouteETAWalkingSeconds != nil || schedule.RouteETABusSeconds != nil || schedule.RouteETAPublicTransitSeconds != nil {
		response.RoutePreview = &integrationScheduleRoutePreviewResponse{
			ImageRef:       schedule.RouteImageRef,
			DistanceMeters: schedule.RouteDistanceMeters,
			ETA: integrationScheduleRouteETAResponse{
				WalkingSeconds:       schedule.RouteETAWalkingSeconds,
				BusSeconds:           schedule.RouteETABusSeconds,
				PublicTransitSeconds: schedule.RouteETAPublicTransitSeconds,
			},
		}
	}

	if schedule.RoutePreviewWarning != nil {
		if strings.TrimSpace(*schedule.RoutePreviewWarning) != "" {
			response.Warnings = append(response.Warnings, strings.TrimSpace(*schedule.RoutePreviewWarning))
		}
	}

	return response
}

func applyRoutePreviewToSchedule(schedule *dbmodel.Schedule, preview *integrationScheduleRoutePreviewRequest) ([]string, error) {
	if schedule == nil || preview == nil {
		return nil, nil
	}

	warnings := []string{}

	if preview.ImageRef != nil {
		ref := strings.TrimSpace(*preview.ImageRef)
		if ref == "" {
			warnings = append(warnings, "route preview image_ref was empty and ignored")
		} else {
			schedule.RouteImageRef = &ref
		}
	}

	if preview.DistanceMeters != nil {
		if *preview.DistanceMeters < 0 {
			return warnings, fmt.Errorf("route preview distance_meters must be >= 0")
		}
		schedule.RouteDistanceMeters = preview.DistanceMeters
	}

	if preview.ETA != nil {
		if preview.ETA.WalkingSeconds != nil {
			if *preview.ETA.WalkingSeconds < 0 {
				return warnings, fmt.Errorf("route preview eta.walking_seconds must be >= 0")
			}
			schedule.RouteETAWalkingSeconds = preview.ETA.WalkingSeconds
		}
		if preview.ETA.BusSeconds != nil {
			if *preview.ETA.BusSeconds < 0 {
				return warnings, fmt.Errorf("route preview eta.bus_seconds must be >= 0")
			}
			schedule.RouteETABusSeconds = preview.ETA.BusSeconds
		}
		if preview.ETA.PublicTransitSeconds != nil {
			if *preview.ETA.PublicTransitSeconds < 0 {
				return warnings, fmt.Errorf("route preview eta.public_transit_seconds must be >= 0")
			}
			schedule.RouteETAPublicTransitSeconds = preview.ETA.PublicTransitSeconds
		}
	}

	return warnings, nil
}

func trimmedStringPtr(value string) *string {
	item := strings.TrimSpace(value)
	if item == "" {
		return nil
	}
	return &item
}

func resolvePreviewPinLabelByID(pinID uint) (string, error) {
	if pinID == 0 {
		return "", fmt.Errorf("pin_id is required")
	}
	var pin dbmodel.Pin
	pin.ID = pinID
	if err := pin.GetById(database.Connection); err != nil {
		return "", err
	}
	return strings.TrimSpace(pin.Label), nil
}

func resolvePreviewPinImagePathByID(pinID uint) (string, error) {
	if pinID == 0 {
		return "", fmt.Errorf("pin_id is required")
	}
	var pin dbmodel.Pin
	pin.ID = pinID
	if err := pin.GetById(database.Connection); err != nil {
		return "", err
	}
	return strings.TrimSpace(pin.ImagePath), nil
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

func authenticateIntegrationRequestJSON(w http.ResponseWriter, r *http.Request, operation string, requiredScope string, queryContext string) (*dbmodel.APIKey, bool) {
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
		_ = integrationCreateAuditLogFn(nil, "unknown", APIKeyAuditEvent{
			Operation: operationWithContext,
			SourceIP:  sourceIP,
			Success:   false,
			Reason:    "missing api key header",
		})
		if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
			writeIntegrationError(w, http.StatusUnauthorized, "deprecated_jwt_auth", "jwt-based automation auth is deprecated for integration endpoints; use API key")
			return nil, false
		}
		writeIntegrationError(w, http.StatusUnauthorized, "missing_api_key", "missing api key")
		return nil, false
	}

	apiKey, err := AuthenticateAPIKey(raw, APIKeyAuditEvent{
		Operation: operationWithContext,
		SourceIP:  sourceIP,
		Success:   false,
		Reason:    "authentication failed",
	})
	if err != nil {
		writeIntegrationError(w, http.StatusUnauthorized, "invalid_api_key", err.Error())
		return nil, false
	}

	if !HasScope(apiKey, requiredScope) {
		_ = integrationCreateAuditLogFn(&apiKey.ID, apiKey.Name, APIKeyAuditEvent{
			Operation: operationWithContext,
			SourceIP:  sourceIP,
			Success:   false,
			Reason:    fmt.Sprintf("missing scope: %s", requiredScope),
		})
		writeIntegrationError(w, http.StatusForbidden, "api_key_scope_denied", (&helper.APIKeyScopeDeniedError{}).Error())
		return nil, false
	}

	log.Printf("api-key request key_id=%d key_name=%s operation=%s scope=%s source_ip=%s", apiKey.ID, apiKey.Name, operationWithContext, requiredScope, sourceIP)
	return apiKey, true
}

func parseIntegrationNearbySearchQuery(r *http.Request) (integrationNearbySearchQuery, error) {
	query := integrationNearbySearchQuery{
		Limit: integrationNearbyDefaultLimit,
	}

	latitudeRaw := strings.TrimSpace(r.URL.Query().Get("latitude"))
	if latitudeRaw == "" {
		return query, fmt.Errorf("latitude is required")
	}
	latitude, err := strconv.ParseFloat(latitudeRaw, 64)
	if err != nil || latitude < -90 || latitude > 90 {
		return query, fmt.Errorf("latitude must be within -90 to 90")
	}
	query.Latitude = latitude

	longitudeRaw := strings.TrimSpace(r.URL.Query().Get("longitude"))
	if longitudeRaw == "" {
		return query, fmt.Errorf("longitude is required")
	}
	longitude, err := strconv.ParseFloat(longitudeRaw, 64)
	if err != nil || longitude < -180 || longitude > 180 {
		return query, fmt.Errorf("longitude must be within -180 to 180")
	}
	query.Longitude = longitude

	radiusRaw := strings.TrimSpace(r.URL.Query().Get("radius"))
	if radiusRaw == "" {
		radiusRaw = strings.TrimSpace(r.URL.Query().Get("area"))
	}
	if radiusRaw == "" {
		return query, fmt.Errorf("radius or area is required")
	}
	radius, err := strconv.ParseFloat(radiusRaw, 64)
	if err != nil || radius <= 0 {
		return query, fmt.Errorf("radius must be a positive number")
	}
	if radius > integrationNearbyMaxRadiusMeters {
		return query, fmt.Errorf("radius exceeds maximum allowed (%.0f meters)", integrationNearbyMaxRadiusMeters)
	}
	query.RadiusMeters = radius

	limitRaw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitRaw != "" {
		limit, convErr := strconv.Atoi(limitRaw)
		if convErr != nil || limit <= 0 {
			return query, fmt.Errorf("limit must be a positive integer")
		}
		if limit > integrationNearbyMaxLimit {
			limit = integrationNearbyMaxLimit
		}
		query.Limit = limit
	}

	return query, nil
}

func findNearbyMarkersByDistance(relation dbmodel.UserRelation, params integrationNearbySearchQuery, includeTesting bool) ([]integrationNearbyMarkerRow, error) {
	current := time.Now().AddDate(0, 0, -1)
	baseQuery := database.Connection.Model(&dbmodel.Marker{}).
		Where("relation_id = ?", relation.ID).
		Where("status != ?", constant.Arrived).
		Where("to_time IS NULL OR (to_time IS NOT NULL AND to_time >= ?)", current.Format(time.RFC3339)).
		Where("type IN (SELECT value FROM marker_types WHERE hidden = FALSE)")
	if !includeTesting {
		baseQuery = baseQuery.Where("testing = ?", false)
	}

	latDelta := params.RadiusMeters / 111320.0
	south := math.Max(-90, params.Latitude-latDelta)
	north := math.Min(90, params.Latitude+latDelta)
	baseQuery = baseQuery.Where("latitude >= ? AND latitude <= ?", south, north)

	lonDelta := 180.0
	cosLat := math.Cos(params.Latitude * math.Pi / 180.0)
	if math.Abs(cosLat) > 1e-12 {
		lonDelta = params.RadiusMeters / (111320.0 * cosLat)
		if lonDelta > 180 {
			lonDelta = 180
		}
	}
	west := normalizeLongitude(params.Longitude - lonDelta)
	east := normalizeLongitude(params.Longitude + lonDelta)
	if west <= east {
		baseQuery = baseQuery.Where("longitude >= ? AND longitude <= ?", west, east)
	} else {
		baseQuery = baseQuery.Where("(longitude >= ? OR longitude <= ?)", west, east)
	}

	distanceExpr := fmt.Sprintf(
		`(%f * acos(least(1.0, greatest(-1.0, cos(radians(?)) * cos(radians(latitude)) * cos(radians(longitude) - radians(?)) + sin(radians(?)) * sin(radians(latitude))))))`,
		integrationNearbyEarthRadiusMeters,
	)

	// NOTE:
	// We intentionally avoid running Preload on a result struct that embeds Marker plus
	// computed columns (distance_meters). On gorm v1.21 this can panic with
	// "reflect: Field index out of range" in preload callbacks.
	type nearbyDistanceRow struct {
		ID             uint    `gorm:"column:id"`
		DistanceMeters float64 `gorm:"column:distance_meters"`
	}

	distanceRows := make([]nearbyDistanceRow, 0)
	if err := baseQuery.
		Select("markers.id AS id, "+distanceExpr+" AS distance_meters", params.Latitude, params.Longitude, params.Latitude).
		Where(distanceExpr+" <= ?", params.Latitude, params.Longitude, params.Latitude, params.RadiusMeters).
		Order("distance_meters asc").
		Order("id asc").
		Limit(params.Limit).
		Scan(&distanceRows).Error; err != nil {
		return nil, err
	}
	if len(distanceRows) == 0 {
		return []integrationNearbyMarkerRow{}, nil
	}

	ids := make([]uint, 0, len(distanceRows))
	distanceByID := make(map[uint]float64, len(distanceRows))
	for _, item := range distanceRows {
		ids = append(ids, item.ID)
		distanceByID[item.ID] = item.DistanceMeters
	}

	markers := make([]dbmodel.Marker, 0, len(ids))
	if err := database.Connection.
		Model(&dbmodel.Marker{}).
		Where("id IN (?)", ids).
		Preload("RestaurantInfo").
		Find(&markers).Error; err != nil {
		return nil, err
	}

	markerByID := make(map[uint]dbmodel.Marker, len(markers))
	for _, marker := range markers {
		markerByID[marker.ID] = marker
	}

	rows := make([]integrationNearbyMarkerRow, 0, len(distanceRows))
	for _, item := range distanceRows {
		marker, ok := markerByID[item.ID]
		if !ok {
			continue
		}
		rows = append(rows, integrationNearbyMarkerRow{
			Marker:         marker,
			DistanceMeters: distanceByID[item.ID],
		})
	}
	return rows, nil
}

func normalizeLongitude(value float64) float64 {
	if value > 180 {
		return value - 360
	}
	if value < -180 {
		return value + 360
	}
	return value
}

func formatAPIKey(apiKey *dbmodel.APIKey) apiKeyResponse {
	var serviceAccountName *string
	if apiKey.ServiceAccount != nil {
		name := strings.TrimSpace(apiKey.ServiceAccount.Name)
		if name != "" {
			serviceAccountName = &name
		}
	}
	return apiKeyResponse{
		ID:                 apiKey.ID,
		Name:               apiKey.Name,
		Testing:            apiKey.Testing,
		Prefix:             apiKey.Prefix,
		Scopes:             apiKey.ScopeList(),
		Status:             apiKey.Status,
		RelationID:         apiKey.RelationID,
		ActorUserID:        apiKey.ActorUserID,
		ServiceAccountID:   apiKey.ServiceAccountID,
		ServiceAccountName: serviceAccountName,
		LastUsedAt:         apiKey.LastUsedAt,
		ExpiresAt:          apiKey.ExpiresAt,
		CreatedAt:          apiKey.CreatedAt,
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
	header := normalizeAuthToken(r.Header.Get("Authorization"))
	if header == "" {
		return nil
	}

	user, err := ValidateToken(header)
	if err != nil {
		log.Printf("failed to validate current user from request: %v", err)
		return nil
	}
	return user
}
