package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mapmarker/backend/database/dbmodel"
)

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
