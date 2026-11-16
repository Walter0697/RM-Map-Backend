package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"mapmarker/backend/database/dbmodel"
)

func TestCreateSocialMediaShortcutSetupHandlerReturnsSetupAndDownloadURLs(t *testing.T) {
	originalUser := socialMediaShortcutCurrentUserFn
	originalRelation := socialMediaShortcutCurrentRelationFn
	originalCreate := socialMediaShortcutCreateSetupTokenFn
	defer func() {
		socialMediaShortcutCurrentUserFn = originalUser
		socialMediaShortcutCurrentRelationFn = originalRelation
		socialMediaShortcutCreateSetupTokenFn = originalCreate
	}()
	socialMediaShortcutCurrentUserFn = func(*http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 7}}
	}
	socialMediaShortcutCurrentRelationFn = func(dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 11}}, nil
	}
	socialMediaShortcutCreateSetupTokenFn = func(uint, uint) (string, error) {
		return "setup-token", nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/settings/ios-shortcut/setup", nil)
	request.Host = "api.example.test"
	request.Header.Set("X-Forwarded-Proto", "https")
	CreateSocialMediaShortcutSetupHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	response := struct {
		SetupURL    string `json:"setup_url"`
		DownloadURL string `json:"download_url"`
	}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.SetupURL != "https://api.example.test/api/shortcut/setup/setup-token" {
		t.Fatalf("unexpected setup URL: %q", response.SetupURL)
	}
	if response.DownloadURL != "https://api.example.test/settings/ios-shortcut/download?token=setup-token" {
		t.Fatalf("unexpected download URL: %q", response.DownloadURL)
	}
}

func TestCreateSocialMediaShortcutSetupHandlerRejectsUnauthenticatedRequest(t *testing.T) {
	originalUser := socialMediaShortcutCurrentUserFn
	defer func() { socialMediaShortcutCurrentUserFn = originalUser }()
	socialMediaShortcutCurrentUserFn = func(*http.Request) *dbmodel.User { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/settings/ios-shortcut/setup", nil)
	CreateSocialMediaShortcutSetupHandler(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestSocialMediaShortcutDownloadReturnsAttachment(t *testing.T) {
	originalUser := socialMediaShortcutCurrentUserFn
	defer func() { socialMediaShortcutCurrentUserFn = originalUser }()
	socialMediaShortcutCurrentUserFn = func(*http.Request) *dbmodel.User {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 7}}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/settings/ios-shortcut/download", nil)
	DownloadSocialMediaShortcutHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Disposition") != `attachment; filename="Roroadmap Analyze.shortcut"` {
		t.Fatalf("unexpected attachment content disposition: %q", recorder.Header().Get("Content-Disposition"))
	}
	if recorder.Header().Get("Content-Type") != "application/x-shortcut" {
		t.Fatalf("expected Apple Shortcut content type, got %q", recorder.Header().Get("Content-Type"))
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("expected non-empty Shortcut artifact")
	}
}

func TestSetupSocialMediaShortcutHandlerReturnsAnalysisEndpointAndKey(t *testing.T) {
	originalExchange := socialMediaShortcutExchangeTokenFn
	defer func() { socialMediaShortcutExchangeTokenFn = originalExchange }()
	socialMediaShortcutExchangeTokenFn = func(token string) (string, error) {
		if token != "setup-token" {
			t.Fatalf("unexpected token %q", token)
		}
		return "rmk_test", nil
	}

	router := chi.NewRouter()
	router.Post("/api/shortcut/setup/{token}", SetupSocialMediaShortcutHandler)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/shortcut/setup/setup-token", nil)
	request.Host = "api.example.test"
	request.Header.Set("X-Forwarded-Proto", "https")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	response := socialMediaShortcutExchangeResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.SubmitURL != "https://api.example.test/integration/social-media/analyses" || response.APIKey != "rmk_test" {
		t.Fatalf("unexpected exchange response: %+v", response)
	}
}

func TestSetupSocialMediaShortcutHandlerRejectsInvalidToken(t *testing.T) {
	originalExchange := socialMediaShortcutExchangeTokenFn
	defer func() { socialMediaShortcutExchangeTokenFn = originalExchange }()
	socialMediaShortcutExchangeTokenFn = func(string) (string, error) {
		return "", ErrInvalidSocialMediaShortcutSetupToken
	}

	router := chi.NewRouter()
	router.Post("/api/shortcut/setup/{token}", SetupSocialMediaShortcutHandler)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/shortcut/setup/expired", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}
