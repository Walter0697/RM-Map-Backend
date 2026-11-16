package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTelegramBotAPIBaseURL = "https://api.telegram.org"

type telegramNotifier struct {
	accessURL     string
	accessToken   string
	botToken      string
	botAPIBaseURL string
	httpClient    *http.Client
}

type telegramLookupResponse struct {
	User       string `json:"user"`
	Name       string `json:"name"`
	TelegramID int64  `json:"telegram_id"`
}

type telegramSendMessageResponse struct {
	OK bool `json:"ok"`
}

func newTelegramNotifier(accessURL, accessToken, botToken, botAPIBaseURL string, timeoutMS int) *telegramNotifier {
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	return &telegramNotifier{
		accessURL:     strings.TrimRight(strings.TrimSpace(accessURL), "/"),
		accessToken:   accessToken,
		botToken:      botToken,
		botAPIBaseURL: strings.TrimRight(strings.TrimSpace(botAPIBaseURL), "/"),
		httpClient:    &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
	}
}

func NotifyMarkerCreated(marker dbmodel.Marker, user dbmodel.User) error {
	return NotifyTelegramMessage(user, fmt.Sprintf("📍 Marker created: %s (#%d)", marker.Label, marker.ID))
}

func NotifyTelegramMessage(user dbmodel.User, message string) error {
	settings := config.Data.TelegramNotification
	if !settings.Enable {
		return nil
	}
	if strings.TrimSpace(settings.AccessURL) == "" || strings.TrimSpace(settings.AccessToken) == "" || strings.TrimSpace(settings.BotToken) == "" {
		return fmt.Errorf("telegram notification configuration is incomplete")
	}
	notifier := newTelegramNotifier(
		settings.AccessURL,
		settings.AccessToken,
		settings.BotToken,
		defaultTelegramBotAPIBaseURL,
		settings.TimeoutMS,
	)
	return notifier.NotifyMessage(user, message)
}

func (n *telegramNotifier) NotifyMarkerCreated(marker dbmodel.Marker, user dbmodel.User) error {
	return n.NotifyMessage(user, fmt.Sprintf("📍 Marker created: %s (#%d)", marker.Label, marker.ID))
}

func (n *telegramNotifier) NotifyMessage(user dbmodel.User, message string) error {
	telegramID, err := n.lookupTelegramID(user.Username)
	if err != nil {
		return err
	}
	if telegramID == 0 {
		return nil
	}

	endpoint := n.botAPIBaseURL + "/bot" + n.botToken + "/sendMessage"
	payload := map[string]any{
		"chat_id": telegramID,
		"text":    message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal Telegram message: %w", err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create Telegram request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := n.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send Telegram notification: request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("send Telegram notification: status %d", response.StatusCode)
	}
	var result telegramSendMessageResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode Telegram response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("Telegram rejected notification")
	}
	return nil
}

func (n *telegramNotifier) lookupTelegramID(userID string) (int64, error) {
	endpoint := n.accessURL + "/lookup?user_id=" + url.QueryEscape(strings.TrimSpace(userID))
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("create Telegram access request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+n.accessToken)
	response, err := n.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("lookup Telegram user: request failed")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return 0, nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("lookup Telegram user: status %d", response.StatusCode)
	}
	var result telegramLookupResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode Telegram lookup response: %w", err)
	}
	if result.TelegramID <= 0 {
		return 0, fmt.Errorf("Telegram lookup returned an invalid user id")
	}
	return result.TelegramID, nil
}
