package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const CalendarProviderGoogle = "google_calendar"

type CalendarProviderOperationError struct {
	Code              string
	Message           string
	Retryable         bool
	ReauthRequired    bool
	ReconciledSuccess bool
}

func (err *CalendarProviderOperationError) Error() string {
	return strings.TrimSpace(err.Message)
}

type GoogleCalendarAdapter struct {
	httpClient *http.Client
}

func NewGoogleCalendarAdapter() *GoogleCalendarAdapter {
	return &GoogleCalendarAdapter{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (adapter *GoogleCalendarAdapter) ProviderKey() string {
	return CalendarProviderGoogle
}

func (adapter *GoogleCalendarAdapter) CreateEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, request CalendarEventUpsertRequest) (CalendarCreateEventResult, error) {
	accessToken, updatedConnection, err := adapter.ensureAccessToken(ctx, connection)
	if err != nil {
		return CalendarCreateEventResult{}, err
	}
	connection = *updatedConnection

	payload := map[string]interface{}{
		"summary":     strings.TrimSpace(request.Title),
		"description": strings.TrimSpace(request.Description),
		"start": map[string]interface{}{
			"dateTime": request.StartAt.UTC().Format(time.RFC3339),
		},
		"end": map[string]interface{}{
			"dateTime": request.StartAt.UTC().Add(2 * time.Hour).Format(time.RFC3339),
		},
	}
	if request.EndAt != nil {
		payload["end"] = map[string]interface{}{
			"dateTime": request.EndAt.UTC().Format(time.RFC3339),
		}
	}
	if strings.TrimSpace(request.Location) != "" {
		payload["location"] = strings.TrimSpace(request.Location)
	}
	if strings.TrimSpace(request.Timezone) != "" {
		payload["start"].(map[string]interface{})["timeZone"] = strings.TrimSpace(request.Timezone)
		payload["end"].(map[string]interface{})["timeZone"] = strings.TrimSpace(request.Timezone)
	}

	apiBase := strings.TrimSuffix(strings.TrimSpace(config.Data.CalendarGoogle.APIBaseURL), "/")
	responseBody, err := adapter.doJSONRequest(ctx, http.MethodPost, apiBase+"/calendars/primary/events", accessToken, payload)
	if err != nil {
		return CalendarCreateEventResult{}, err
	}
	response := struct {
		ID         string `json:"id"`
		CalendarID string `json:"organizer.email"`
	}{}
	if unmarshalErr := json.Unmarshal(responseBody, &response); unmarshalErr != nil {
		return CalendarCreateEventResult{}, unmarshalErr
	}
	if strings.TrimSpace(response.ID) == "" {
		return CalendarCreateEventResult{}, &CalendarProviderOperationError{
			Code:      "provider_invalid_response",
			Message:   "google create event response missing id",
			Retryable: false,
		}
	}
	return CalendarCreateEventResult{
		ExternalEventID:    strings.TrimSpace(response.ID),
		ExternalCalendarID: "primary",
	}, nil
}

func (adapter *GoogleCalendarAdapter) UpdateEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, externalEventID string, request CalendarEventUpsertRequest) error {
	accessToken, updatedConnection, err := adapter.ensureAccessToken(ctx, connection)
	if err != nil {
		return err
	}
	connection = *updatedConnection

	payload := map[string]interface{}{
		"summary":     strings.TrimSpace(request.Title),
		"description": strings.TrimSpace(request.Description),
		"start": map[string]interface{}{
			"dateTime": request.StartAt.UTC().Format(time.RFC3339),
		},
		"end": map[string]interface{}{
			"dateTime": request.StartAt.UTC().Add(2 * time.Hour).Format(time.RFC3339),
		},
	}
	if request.EndAt != nil {
		payload["end"] = map[string]interface{}{
			"dateTime": request.EndAt.UTC().Format(time.RFC3339),
		}
	}
	if strings.TrimSpace(request.Location) != "" {
		payload["location"] = strings.TrimSpace(request.Location)
	}
	if strings.TrimSpace(request.Timezone) != "" {
		payload["start"].(map[string]interface{})["timeZone"] = strings.TrimSpace(request.Timezone)
		payload["end"].(map[string]interface{})["timeZone"] = strings.TrimSpace(request.Timezone)
	}

	apiBase := strings.TrimSuffix(strings.TrimSpace(config.Data.CalendarGoogle.APIBaseURL), "/")
	_, err = adapter.doJSONRequest(ctx, http.MethodPut, apiBase+"/calendars/primary/events/"+url.PathEscape(strings.TrimSpace(externalEventID)), accessToken, payload)
	return err
}

func (adapter *GoogleCalendarAdapter) DeleteEvent(ctx context.Context, connection dbmodel.CalendarProviderConnection, externalEventID string) error {
	accessToken, _, err := adapter.ensureAccessToken(ctx, connection)
	if err != nil {
		return err
	}
	apiBase := strings.TrimSuffix(strings.TrimSpace(config.Data.CalendarGoogle.APIBaseURL), "/")
	_, err = adapter.doJSONRequest(ctx, http.MethodDelete, apiBase+"/calendars/primary/events/"+url.PathEscape(strings.TrimSpace(externalEventID)), accessToken, nil)
	return err
}

func (adapter *GoogleCalendarAdapter) ensureAccessToken(ctx context.Context, connection dbmodel.CalendarProviderConnection) (string, *dbmodel.CalendarProviderConnection, error) {
	accessToken, err := decryptCalendarSecret(connection.AccessTokenEncrypted)
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	if strings.TrimSpace(accessToken) != "" && (connection.TokenExpiresAt == nil || connection.TokenExpiresAt.After(now.Add(90*time.Second))) {
		return accessToken, &connection, nil
	}

	refreshToken, err := decryptCalendarSecret(connection.RefreshTokenEncrypted)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(refreshToken) == "" {
		return "", nil, &CalendarProviderOperationError{
			Code:           "reauthorization_required",
			Message:        "google refresh token is unavailable",
			ReauthRequired: true,
		}
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", config.Data.CalendarGoogle.ClientID)
	form.Set("client_secret", config.Data.CalendarGoogle.ClientSecret)
	form.Set("refresh_token", refreshToken)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Data.CalendarGoogle.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := adapter.httpClient.Do(request)
	if err != nil {
		return "", nil, &CalendarProviderOperationError{
			Code:      "provider_network_error",
			Message:   err.Error(),
			Retryable: true,
		}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized {
			return "", nil, &CalendarProviderOperationError{
				Code:           "reauthorization_required",
				Message:        fmt.Sprintf("google refresh token rejected (status=%d)", response.StatusCode),
				ReauthRequired: true,
			}
		}
		return "", nil, &CalendarProviderOperationError{
			Code:      "provider_refresh_failed",
			Message:   fmt.Sprintf("google refresh failed with status %d", response.StatusCode),
			Retryable: response.StatusCode >= 500,
		}
	}
	tokenResponse := struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}{}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return "", nil, &CalendarProviderOperationError{
			Code:      "provider_refresh_failed",
			Message:   "google refresh response missing access_token",
			Retryable: false,
		}
	}
	encryptedToken, err := encryptCalendarSecret(tokenResponse.AccessToken)
	if err != nil {
		return "", nil, err
	}
	expireAt := now.Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	connection.AccessTokenEncrypted = encryptedToken
	connection.TokenExpiresAt = &expireAt
	connection.LastRefreshAt = &now
	if saveErr := database.Connection.Save(&connection).Error; saveErr != nil {
		return "", nil, saveErr
	}
	return tokenResponse.AccessToken, &connection, nil
}

func (adapter *GoogleCalendarAdapter) doJSONRequest(ctx context.Context, method string, rawURL string, accessToken string, bodyPayload interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if bodyPayload != nil {
		rawBody, err := json.Marshal(bodyPayload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(rawBody)
	}

	request, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	if bodyPayload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := adapter.httpClient.Do(request)
	if err != nil {
		return nil, &CalendarProviderOperationError{
			Code:      "provider_network_error",
			Message:   err.Error(),
			Retryable: true,
		}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return body, nil
	}

	if response.StatusCode == http.StatusNotFound && method == http.MethodDelete {
		return nil, &CalendarProviderOperationError{
			Code:              "external_event_not_found",
			Message:           "google event already removed",
			ReconciledSuccess: true,
		}
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, &CalendarProviderOperationError{
			Code:           "reauthorization_required",
			Message:        fmt.Sprintf("google api authorization failed (status=%d)", response.StatusCode),
			ReauthRequired: true,
		}
	}
	return nil, &CalendarProviderOperationError{
		Code:      "provider_request_failed",
		Message:   fmt.Sprintf("google api request failed status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body))),
		Retryable: response.StatusCode >= 500,
	}
}
