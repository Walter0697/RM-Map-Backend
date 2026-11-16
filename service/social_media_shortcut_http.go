package service

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
)

type socialMediaShortcutSetupResponse struct {
	SetupURL    string `json:"setup_url"`
	DownloadURL string `json:"download_url"`
}

type socialMediaShortcutExchangeResponse struct {
	SubmitURL string `json:"submit_url"`
	APIKey    string `json:"api_key"`
}

var (
	socialMediaShortcutCurrentUserFn      = currentUserFromRequest
	socialMediaShortcutCurrentRelationFn  = GetCurrentRelation
	socialMediaShortcutCreateSetupTokenFn = CreateSocialMediaShortcutSetupToken
	socialMediaShortcutExchangeTokenFn    = ExchangeSocialMediaShortcutSetupToken
)

func socialMediaShortcutRequestOrigin(r *http.Request) string {
	scheme := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if scheme == "" {
		scheme = "http"
	}
	host := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func CreateSocialMediaShortcutSetupHandler(w http.ResponseWriter, r *http.Request) {
	user := socialMediaShortcutCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	relation, err := socialMediaShortcutCurrentRelationFn(*user)
	if err != nil {
		http.Error(w, "failed to resolve current relation", http.StatusInternalServerError)
		return
	}
	if relation == nil || relation.ID == 0 {
		http.Error(w, "a current relation is required", http.StatusBadRequest)
		return
	}
	origin := socialMediaShortcutRequestOrigin(r)
	if origin == "" {
		http.Error(w, "missing request host", http.StatusBadRequest)
		return
	}
	token, err := socialMediaShortcutCreateSetupTokenFn(user.ID, relation.ID)
	if err != nil {
		http.Error(w, "failed to create shortcut setup", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, socialMediaShortcutSetupResponse{
		SetupURL:    origin + "/api/shortcut/setup/" + token,
		DownloadURL: origin + "/settings/ios-shortcut/download?token=" + url.QueryEscape(token),
	})
}

func DownloadSocialMediaShortcutHandler(w http.ResponseWriter, r *http.Request) {
	if socialMediaShortcutCurrentUserFn(r) == nil {
		valid, err := ValidateSocialMediaShortcutSetupToken(r.URL.Query().Get("token"))
		if err != nil {
			http.Error(w, "failed to validate shortcut download", http.StatusInternalServerError)
			return
		}
		if !valid {
			http.Error(w, "permission denied", http.StatusUnauthorized)
			return
		}
	}
	_, sourceFile, _, _ := runtime.Caller(0)
	artifactPath := filepath.Join(filepath.Dir(sourceFile), "..", "assets", "roroadmap-ios-shortcut.shortcut")
	contents, err := os.ReadFile(artifactPath)
	if err != nil {
		http.Error(w, "shortcut artifact unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-shortcut")
	w.Header().Set("Content-Disposition", `attachment; filename="Roroadmap Analyze.shortcut"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(contents)
}

func SetupSocialMediaShortcutHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		http.Error(w, "invalid or expired setup token", http.StatusUnauthorized)
		return
	}
	rawAPIKey, err := socialMediaShortcutExchangeTokenFn(token)
	if err != nil {
		if errors.Is(err, ErrInvalidSocialMediaShortcutSetupToken) {
			http.Error(w, "invalid or expired setup token", http.StatusUnauthorized)
			return
		}
		http.Error(w, "shortcut setup failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, socialMediaShortcutExchangeResponse{
		SubmitURL: socialMediaShortcutRequestOrigin(r) + "/integration/social-media/analyses",
		APIKey:    rawAPIKey,
	})
}

func ListSocialMediaShortcutAPIKeysHandler(w http.ResponseWriter, r *http.Request) {
	user := socialMediaShortcutCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	keys, err := ListSocialMediaShortcutAPIKeys(user.ID)
	if err != nil {
		http.Error(w, "failed to load shortcut API keys", http.StatusInternalServerError)
		return
	}
	items := make([]apiKeyResponse, 0, len(keys))
	for index := range keys {
		items = append(items, formatAPIKey(&keys[index]))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func RevokeSocialMediaShortcutAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	user := socialMediaShortcutCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	key, err := RevokeSocialMediaShortcutAPIKey(uint(id), user.ID, user)
	if err != nil {
		http.Error(w, "shortcut API key not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"api_key": formatAPIKey(key)})
}
