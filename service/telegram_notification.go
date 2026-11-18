package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
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

var telegramNotificationTracerFn = func() socialAnalysisTracerProvider {
	return socialAnalysisTracerFn()
}

var markerCreatedPreviewFn = GenerateStaticMapPreviewByUsername

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
	return notifyTelegramMarkerCreated(marker, user)
}

func NotifyTelegramMessage(user dbmodel.User, message string) error {
	return notifyTelegramMessage(user, message, "message")
}

func resolveTelegramNotificationSecrets(ctx context.Context, settings config.TelegramNotificationSetting, resolver SecretResolver) (string, string, error) {
	resolve := func(name, configured string) (string, error) {
		value := strings.TrimSpace(configured)
		if resolver != nil {
			resolved, err := resolver.Resolve(ctx, name)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(resolved) != "" {
				value = strings.TrimSpace(resolved)
			}
		}
		if value == "" {
			value = strings.TrimSpace(os.Getenv(name))
		}
		return value, nil
	}

	accessToken, err := resolve("TELEGRAM_ACCESS_TOKEN", settings.AccessToken)
	if err != nil {
		return "", "", err
	}
	botToken, err := resolve("TELEGRAM_BOT_TOKEN", settings.BotToken)
	if err != nil {
		return "", "", err
	}
	return accessToken, botToken, nil
}

func notifyTelegramMessage(user dbmodel.User, message string, event string) error {
	return notifyTelegramMessageContext(context.Background(), user, message, event)
}

func notifyTelegramMessageContext(ctx context.Context, user dbmodel.User, message string, event string) error {
	settings := config.Data.TelegramNotification
	if !settings.Enable {
		return nil
	}
	return traceTelegramNotificationContext(ctx, event, user, func() error {
		notifier, err := configuredTelegramNotifier(settings)
		if err != nil {
			return err
		}
		return notifier.NotifyMessage(user, message)
	})
}

func notifyTelegramMarkerCreated(marker dbmodel.Marker, user dbmodel.User) error {
	return notifyTelegramMarkerCreatedContext(context.Background(), marker, user)
}

func notifyTelegramMarkerCreatedContext(ctx context.Context, marker dbmodel.Marker, user dbmodel.User) error {
	settings := config.Data.TelegramNotification
	if !settings.Enable {
		return nil
	}
	return traceTelegramNotificationContext(ctx, "marker_created", user, func() error {
		notifier, err := configuredTelegramNotifier(settings)
		if err != nil {
			return err
		}
		return notifier.NotifyMarkerCreated(marker, user)
	})
}

func configuredTelegramNotifier(settings config.TelegramNotificationSetting) (*telegramNotifier, error) {
	var secretResolver SecretResolver
	if config.Data.Infisical.Enable {
		secretResolver = NewSecretResolver(config.Data.Infisical, http.DefaultClient)
	}
	accessToken, botToken, err := resolveTelegramNotificationSecrets(
		context.Background(),
		settings,
		secretResolver,
	)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(settings.AccessURL) == "" || accessToken == "" || botToken == "" {
		return nil, fmt.Errorf("telegram notification configuration is incomplete")
	}
	return newTelegramNotifier(
		settings.AccessURL,
		accessToken,
		botToken,
		defaultTelegramBotAPIBaseURL,
		settings.TimeoutMS,
	), nil
}

func traceTelegramNotification(event string, user dbmodel.User, operation func() error) error {
	return traceTelegramNotificationContext(context.Background(), event, user, operation)
}

func traceTelegramNotificationContext(ctx context.Context, event string, user dbmodel.User, operation func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	tracer := telegramNotificationTracerFn()
	_, span := tracer.Start(ctx, "telegram.notification", map[string]string{
		"notification.channel": "telegram",
		"notification.event":   strings.TrimSpace(event),
		"telegram.user_id":     strings.TrimSpace(user.Username),
	})
	defer span.End()

	err := operation()
	if err != nil {
		span.SetAttribute("notification.outcome", "failed")
		span.RecordError(err)
		span.SetStatus(false, "telegram notification failed")
		return err
	}
	span.SetAttribute("notification.outcome", "sent_or_skipped")
	span.SetStatus(true, "telegram notification completed")
	return nil
}

func (n *telegramNotifier) NotifyMarkerCreated(marker dbmodel.Marker, user dbmodel.User) error {
	telegramID, err := n.lookupTelegramID(user.Username)
	if err != nil {
		return err
	}
	if telegramID == 0 {
		return nil
	}

	preview, previewErr := markerCreatedPreviewFn(user.Username, marker.Type, marker.Latitude, marker.Longitude)
	if previewErr != nil {
		log.Printf("marker Telegram preview failed marker_id=%d actor=%s err=%v", marker.ID, strings.TrimSpace(user.Username), previewErr)
	} else if err := n.sendPhoto(telegramID, preview.Image, fmt.Sprintf("📍 %s", marker.Label)); err != nil {
		log.Printf("marker Telegram photo failed marker_id=%d actor=%s err=%v", marker.ID, strings.TrimSpace(user.Username), err)
	}

	return n.sendMessageToChat(telegramID, fmt.Sprintf("📍 Marker created: %s (#%d)", marker.Label, marker.ID))
}

func (n *telegramNotifier) NotifyMessage(user dbmodel.User, message string) error {
	telegramID, err := n.lookupTelegramID(user.Username)
	if err != nil {
		return err
	}
	if telegramID == 0 {
		return nil
	}
	return n.sendMessageToChat(telegramID, message)
}

func (n *telegramNotifier) sendMessageToChat(telegramID int64, message string) error {

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

func (n *telegramNotifier) sendPhoto(telegramID int64, imageBytes []byte, caption string) error {
	if len(imageBytes) == 0 {
		return fmt.Errorf("marker preview image is empty")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(telegramID, 10)); err != nil {
		return fmt.Errorf("write Telegram photo chat id: %w", err)
	}
	if err := writer.WriteField("caption", caption); err != nil {
		return fmt.Errorf("write Telegram photo caption: %w", err)
	}
	part, err := writer.CreateFormFile("photo", "marker-preview.png")
	if err != nil {
		return fmt.Errorf("create Telegram photo field: %w", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return fmt.Errorf("write Telegram photo: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close Telegram photo request: %w", err)
	}

	endpoint := n.botAPIBaseURL + "/bot" + n.botToken + "/sendPhoto"
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, &body)
	if err != nil {
		return fmt.Errorf("create Telegram photo request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := n.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send Telegram photo: request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("send Telegram photo: status %d", response.StatusCode)
	}
	var result telegramSendMessageResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode Telegram photo response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("Telegram rejected photo notification")
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
