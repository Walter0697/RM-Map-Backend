package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi"
)

type calendarOAuthStateValue struct {
	UserID   uint
	ExpireAt time.Time
}

type calendarSyncNowRequest struct {
	Provider string `json:"provider"`
}

var calendarOAuthStateStore sync.Map

func CalendarProviderStatusHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	items := make([]dbmodel.CalendarProviderConnection, 0)
	if err := database.Connection.Where("user_id = ?", user.ID).Order("provider_key asc").Find(&items).Error; err != nil {
		http.Error(w, "failed to load calendar connections", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items":   items,
		"metrics": calendarSyncMetricsSnapshot(),
	})
}

func CalendarDisconnectProviderHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	providerKey := normalizeCalendarProviderKey(chi.URLParam(r, "provider"))
	if providerKey == "" {
		http.Error(w, "invalid provider", http.StatusBadRequest)
		return
	}

	runtime := getCalendarSyncRuntime()
	connection, err := runtime.connectionSvc.GetByUserAndProvider(user.ID, providerKey)
	if err != nil {
		http.Error(w, "provider connection not found", http.StatusNotFound)
		return
	}
	if err := runtime.connectionSvc.MarkDisconnected(connection.ID); err != nil {
		http.Error(w, "failed to disconnect provider connection", http.StatusInternalServerError)
		return
	}

	links := make([]dbmodel.ScheduleCalendarSyncLink, 0)
	if err := database.Connection.Where("connection_id = ?", connection.ID).Find(&links).Error; err != nil {
		http.Error(w, "failed to load schedule links", http.StatusInternalServerError)
		return
	}
	updatedCount := 0
	for _, link := range links {
		if err := runtime.linkRepo.MarkDisconnected(link.ID); err != nil {
			log.Printf("[calendar-sync] disconnect_link_failed provider=%s connection_id=%d link_id=%d error=%v", providerKey, connection.ID, link.ID, err)
			continue
		}
		updatedCount++
	}

	calendarAudit("provider_disconnected", map[string]string{
		"provider": providerKey,
		"user_id":  fmt.Sprintf("%d", user.ID),
		"links":    fmt.Sprintf("%d", updatedCount),
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"provider":      providerKey,
		"status":        dbmodel.CalendarConnectionStatusDisconnected,
		"links_updated": updatedCount,
	})
}

func CalendarScheduleSyncStatusHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rawIDs := strings.TrimSpace(r.URL.Query().Get("ids"))
	if rawIDs == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}
	scheduleIDs := make([]uint, 0)
	for _, token := range strings.Split(rawIDs, ",") {
		parsed, err := strconv.ParseUint(strings.TrimSpace(token), 10, 64)
		if err != nil || parsed == 0 {
			continue
		}
		scheduleIDs = append(scheduleIDs, uint(parsed))
	}
	if len(scheduleIDs) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{"items": []map[string]interface{}{}})
		return
	}

	links := make([]dbmodel.ScheduleCalendarSyncLink, 0)
	if err := database.Connection.Where("schedule_id IN ?", scheduleIDs).Find(&links).Error; err != nil {
		http.Error(w, "failed to load schedule sync status", http.StatusInternalServerError)
		return
	}
	items := make([]map[string]interface{}, 0)
	for _, link := range links {
		if !calendarCanAccessSchedule(user.ID, link.ScheduleID) {
			continue
		}
		connection := dbmodel.CalendarProviderConnection{}
		_ = database.Connection.Where("id = ?", link.ConnectionID).First(&connection).Error
		items = append(items, map[string]interface{}{
			"schedule_id":        link.ScheduleID,
			"provider_key":       link.ProviderKey,
			"sync_status":        link.SyncStatus,
			"external_event_id":  link.ExternalEventID,
			"last_error_code":    link.LastErrorCode,
			"last_error_message": link.LastErrorMessage,
			"connection_status":  connection.Status,
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func CalendarGoogleConnectHandler(w http.ResponseWriter, r *http.Request) {
	if !config.Data.CalendarGoogle.Enable {
		http.Error(w, "google calendar integration disabled", http.StatusNotFound)
		return
	}
	user := currentUserFromRequest(r)
	if user == nil {
		tokenQuery := normalizeAuthToken(r.URL.Query().Get("token"))
		if tokenQuery != "" {
			if fromToken, err := ValidateToken(tokenQuery); err == nil {
				user = fromToken
			}
		}
	}
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	state := fmt.Sprintf("gcal-%d-%d", user.ID, time.Now().UnixNano())
	calendarOAuthStateStore.Store(state, calendarOAuthStateValue{
		UserID:   user.ID,
		ExpireAt: time.Now().UTC().Add(10 * time.Minute),
	})

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", config.Data.CalendarGoogle.ClientID)
	params.Set("redirect_uri", config.Data.CalendarGoogle.RedirectURL)
	params.Set("scope", strings.Join(config.Data.CalendarGoogle.Scopes, " "))
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("state", state)

	target := strings.TrimSpace(config.Data.CalendarGoogle.AuthEndpoint) + "?" + params.Encode()
	http.Redirect(w, r, target, http.StatusFound)
}

func CalendarGoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if !config.Data.CalendarGoogle.Enable {
		http.Error(w, "google calendar integration disabled", http.StatusNotFound)
		return
	}

	query := r.URL.Query()
	state := strings.TrimSpace(query.Get("state"))
	code := strings.TrimSpace(query.Get("code"))
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}
	stateValueRaw, ok := calendarOAuthStateStore.Load(state)
	if !ok {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	calendarOAuthStateStore.Delete(state)
	stateValue := stateValueRaw.(calendarOAuthStateValue)
	if stateValue.ExpireAt.Before(time.Now().UTC()) {
		http.Error(w, "expired oauth state", http.StatusBadRequest)
		return
	}

	tokenResponse, err := exchangeGoogleCalendarCode(r.Context(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	encryptedAccess, err := encryptCalendarSecret(tokenResponse.AccessToken)
	if err != nil {
		http.Error(w, "failed to encrypt access token", http.StatusInternalServerError)
		return
	}
	encryptedRefresh, err := encryptCalendarSecret(tokenResponse.RefreshToken)
	if err != nil {
		http.Error(w, "failed to encrypt refresh token", http.StatusInternalServerError)
		return
	}

	var tokenExpiresAt *time.Time
	if tokenResponse.ExpiresIn > 0 {
		expires := time.Now().UTC().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
		tokenExpiresAt = &expires
	}
	connectionSvc := getCalendarSyncRuntime().connectionSvc
	_, err = connectionSvc.UpsertAuthorizedConnection(CalendarConnectionUpsertInput{
		UserID:                stateValue.UserID,
		ProviderKey:           CalendarProviderGoogle,
		GrantedScopes:         splitScopes(tokenResponse.Scope),
		AccessTokenEncrypted:  encryptedAccess,
		RefreshTokenEncrypted: encryptedRefresh,
		TokenExpiresAt:        tokenExpiresAt,
	})
	if err != nil {
		http.Error(w, "failed to persist provider connection", http.StatusInternalServerError)
		return
	}
	calendarAudit("provider_connected", map[string]string{
		"provider": CalendarProviderGoogle,
		"user_id":  fmt.Sprintf("%d", stateValue.UserID),
	})

	redirectURL := strings.TrimSpace(config.Data.CalendarGoogle.FrontendRedirectURL)
	if redirectURL == "" {
		redirectURL = strings.TrimSuffix(config.Data.App.AllowedOrigin, "/") + "/schedules"
	}
	target, parseErr := url.Parse(redirectURL)
	if parseErr != nil {
		http.Error(w, "invalid calendar frontend redirect URL", http.StatusInternalServerError)
		return
	}
	values := target.Query()
	values.Set("calendar_provider", CalendarProviderGoogle)
	values.Set("calendar_connected", "true")
	target.RawQuery = values.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func CalendarSyncNowHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID, ok := calendarScheduleIDFromRequest(w, r)
	if !ok {
		return
	}
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !calendarCanAccessSchedule(user.ID, scheduleID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	request := calendarSyncNowRequest{Provider: CalendarProviderGoogle}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	providerKey := strings.TrimSpace(request.Provider)
	if providerKey == "" {
		providerKey = CalendarProviderGoogle
	}
	result, err := executeManualCalendarSync(r.Context(), user.ID, scheduleID, providerKey, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   result.SyncStatus,
		"action":   result.Action,
		"schedule": scheduleID,
		"provider": providerKey,
		"result":   result,
	})
}

func CalendarRetrySyncHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID, ok := calendarScheduleIDFromRequest(w, r)
	if !ok {
		return
	}
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !calendarCanAccessSchedule(user.ID, scheduleID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	result, err := executeManualCalendarSync(r.Context(), user.ID, scheduleID, CalendarProviderGoogle, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   result.SyncStatus,
		"action":   result.Action,
		"schedule": scheduleID,
		"provider": CalendarProviderGoogle,
		"result":   result,
	})
}

func CalendarDisconnectSyncHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID, ok := calendarScheduleIDFromRequest(w, r)
	if !ok {
		return
	}
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !calendarCanAccessSchedule(user.ID, scheduleID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	result, err := executeManualCalendarSync(r.Context(), user.ID, scheduleID, CalendarProviderGoogle, dbmodel.CalendarSyncJobActionDelete)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   result.SyncStatus,
		"action":   result.Action,
		"schedule": scheduleID,
		"provider": CalendarProviderGoogle,
		"result":   result,
	})
}

func exchangeGoogleCalendarCode(ctx context.Context, code string) (*googleTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", strings.TrimSpace(code))
	form.Set("client_id", strings.TrimSpace(config.Data.CalendarGoogle.ClientID))
	form.Set("client_secret", strings.TrimSpace(config.Data.CalendarGoogle.ClientSecret))
	form.Set("redirect_uri", strings.TrimSpace(config.Data.CalendarGoogle.RedirectURL))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(config.Data.CalendarGoogle.TokenEndpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("google token exchange failed status=%d", response.StatusCode)
	}
	tokenResponse := &googleTokenResponse{}
	if err := json.Unmarshal(body, tokenResponse); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return nil, fmt.Errorf("google token exchange missing access token")
	}
	return tokenResponse, nil
}

type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
}

func splitScopes(rawScopes string) []string {
	fields := strings.Fields(strings.TrimSpace(rawScopes))
	result := make([]string, 0, len(fields))
	for _, item := range fields {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func calendarScheduleIDFromRequest(w http.ResponseWriter, r *http.Request) (uint, bool) {
	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		http.Error(w, "invalid schedule id", http.StatusBadRequest)
		return 0, false
	}
	return uint(id), true
}

func calendarCanAccessSchedule(userID uint, scheduleID uint) bool {
	schedule := dbmodel.Schedule{}
	schedule.ID = scheduleID
	if err := schedule.GetById(database.Connection); err != nil {
		return false
	}
	relation := dbmodel.UserRelation{}
	relation.ID = schedule.RelationId
	if err := relation.GetRelationById(database.Connection); err != nil {
		return false
	}
	return relation.UserOneUID == userID || relation.UserTwoUID == userID
}
