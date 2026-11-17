package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
)

type telegramTestSecretResolver map[string]string

func (resolver telegramTestSecretResolver) Resolve(_ context.Context, name string) (string, error) {
	return resolver[name], nil
}

func TestResolveTelegramNotificationSecretsFromInfisical(t *testing.T) {
	settings := config.TelegramNotificationSetting{}
	resolver := telegramTestSecretResolver{
		"TELEGRAM_ACCESS_TOKEN": "infisical-access-token",
		"TELEGRAM_BOT_TOKEN":    "infisical-bot-token",
	}

	accessToken, botToken, err := resolveTelegramNotificationSecrets(context.Background(), settings, resolver)
	if err != nil {
		t.Fatalf("resolveTelegramNotificationSecrets returned error: %v", err)
	}
	if accessToken != "infisical-access-token" || botToken != "infisical-bot-token" {
		t.Fatalf("unexpected Telegram secrets: access=%q bot=%q", accessToken, botToken)
	}
}

func TestTelegramNotificationSendsMarkerCreatedMessage(t *testing.T) {
	var gotAuthorization string
	var gotMessage struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
	}

	access := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		if r.URL.Path != "/lookup" || r.URL.Query().Get("user_id") != "walter" {
			t.Fatalf("unexpected lookup request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": "walter", "name": "Walter", "telegram_id": int64(42),
		})
	}))
	defer access.Close()

	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botbot-token/sendMessage" {
			t.Fatalf("unexpected Telegram request path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotMessage); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer bot.Close()

	notifier := newTelegramNotifier(access.URL, "access-token", "bot-token", bot.URL, 5000)
	err := notifier.NotifyMarkerCreated(dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 7}}, Label: "Cafe"}, dbmodel.User{Username: "walter"})
	if err != nil {
		t.Fatalf("NotifyMarkerCreated returned error: %v", err)
	}
	if gotAuthorization != "Bearer access-token" {
		t.Fatalf("authorization = %q", gotAuthorization)
	}
	if gotMessage.ChatID != 42 || !strings.Contains(gotMessage.Text, "Cafe") {
		t.Fatalf("message = %+v", gotMessage)
	}
}

func TestTelegramNotificationSendsMarkerCreatedMapPreview(t *testing.T) {
	var gotPhoto bool
	var gotMessage bool
	previousPreviewFn := markerCreatedPreviewFn
	markerCreatedPreviewFn = func(string, string, float64, float64) (*staticPreviewResult, error) {
		return &staticPreviewResult{Image: []byte("preview-png")}, nil
	}
	t.Cleanup(func() { markerCreatedPreviewFn = previousPreviewFn })

	access := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" || r.URL.Query().Get("user_id") != "walter" {
			t.Fatalf("unexpected lookup request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": "walter", "name": "Walter", "telegram_id": int64(42),
		})
	}))
	defer access.Close()

	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/botbot-token/sendPhoto":
			gotPhoto = true
		case "/botbot-token/sendMessage":
			gotMessage = true
		default:
			t.Fatalf("unexpected Telegram request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer bot.Close()

	notifier := newTelegramNotifier(access.URL, "access-token", "bot-token", bot.URL, 5000)
	err := notifier.NotifyMarkerCreated(dbmodel.Marker{
		ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 7}},
		Label:      "Cafe",
	}, dbmodel.User{Username: "walter"})
	if err != nil {
		t.Fatalf("NotifyMarkerCreated returned error: %v", err)
	}
	if !gotPhoto || !gotMessage {
		t.Fatalf("expected map preview and message notifications, photo=%t message=%t", gotPhoto, gotMessage)
	}
}

func TestTelegramNotificationFallsBackWhenMarkerMapPreviewFails(t *testing.T) {
	previousPreviewFn := markerCreatedPreviewFn
	markerCreatedPreviewFn = func(string, string, float64, float64) (*staticPreviewResult, error) {
		return nil, fmt.Errorf("preview unavailable")
	}
	t.Cleanup(func() { markerCreatedPreviewFn = previousPreviewFn })

	var gotMessage bool
	access := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"telegram_id": int64(42)})
	}))
	defer access.Close()

	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botbot-token/sendMessage" {
			t.Fatalf("unexpected Telegram request path: %s", r.URL.Path)
		}
		gotMessage = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer bot.Close()

	notifier := newTelegramNotifier(access.URL, "access-token", "bot-token", bot.URL, 5000)
	if err := notifier.NotifyMarkerCreated(dbmodel.Marker{Label: "Cafe"}, dbmodel.User{Username: "walter"}); err != nil {
		t.Fatalf("NotifyMarkerCreated returned error: %v", err)
	}
	if !gotMessage {
		t.Fatal("expected text notification fallback")
	}
}

func TestTelegramNotificationSkipsUnlinkedUser(t *testing.T) {
	access := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer access.Close()

	notifier := newTelegramNotifier(access.URL, "access-token", "bot-token", "http://127.0.0.1:1", 5000)
	if err := notifier.NotifyMarkerCreated(dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 7}}}, dbmodel.User{Username: "walter"}); err != nil {
		t.Fatalf("unlinked user should be skipped: %v", err)
	}
}

func TestTelegramNotificationDoesNotExposeSecretsOnFailure(t *testing.T) {
	access := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"telegram_id":42}`))
	}))
	defer access.Close()

	notifier := newTelegramNotifier(access.URL, "access-secret", "bot-secret", "http://127.0.0.1:1", 50)
	err := notifier.NotifyMarkerCreated(dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 7}}}, dbmodel.User{Username: "walter"})
	if err == nil {
		t.Fatal("expected Telegram request failure")
	}
	if strings.Contains(err.Error(), "access-secret") || strings.Contains(err.Error(), "bot-secret") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestTelegramNotificationCreatesPhoenixSpan(t *testing.T) {
	tracer := &recordingObservabilityTracer{}
	previousTracer := telegramNotificationTracerFn
	telegramNotificationTracerFn = func() socialAnalysisTracerProvider { return tracer }
	t.Cleanup(func() { telegramNotificationTracerFn = previousTracer })

	err := traceTelegramNotification("review_conflict", dbmodel.User{Username: "walter"}, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("traceTelegramNotification returned error: %v", err)
	}
	if len(tracer.spans) != 1 {
		t.Fatalf("expected one Phoenix span, got %d", len(tracer.spans))
	}
	span := tracer.spans[0]
	if span.name != "telegram.notification" || span.attributes["notification.event"] != "review_conflict" || span.attributes["telegram.user_id"] != "walter" || span.attributes["notification.outcome"] != "sent_or_skipped" || !span.ok {
		t.Fatalf("unexpected Telegram span: %#v", span)
	}
}

func TestTelegramNotificationPhoenixSpanRecordsFailure(t *testing.T) {
	tracer := &recordingObservabilityTracer{}
	previousTracer := telegramNotificationTracerFn
	telegramNotificationTracerFn = func() socialAnalysisTracerProvider { return tracer }
	t.Cleanup(func() { telegramNotificationTracerFn = previousTracer })

	err := traceTelegramNotification("failed", dbmodel.User{Username: "walter"}, func() error {
		return fmt.Errorf("telegram unavailable")
	})
	if err == nil {
		t.Fatal("expected notification error")
	}
	if len(tracer.spans) != 1 || tracer.spans[0].attributes["notification.outcome"] != "failed" || tracer.spans[0].ok {
		t.Fatalf("expected failed Telegram span: %#v", tracer.spans)
	}
}
