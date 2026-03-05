package service

import (
	"encoding/json"
	"net/http"
)

type updateIOSShortcutInstallURLRequest struct {
	IOSShortcutInstallURL *string `json:"ios_shortcut_install_url"`
}

type iosShortcutInstallURLResponse struct {
	IOSShortcutInstallURL string `json:"ios_shortcut_install_url,omitempty"`
}

func SettingsGetIOSShortcutInstallURLHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	shortcutURL, present, err := GetIOSShortcutInstallURL()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := iosShortcutInstallURLResponse{}
	if present {
		response.IOSShortcutInstallURL = shortcutURL
	}
	respondJSON(w, http.StatusOK, response)
}

func AdminGetIOSShortcutInstallURLHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	shortcutURL, present, err := GetIOSShortcutInstallURL()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := iosShortcutInstallURLResponse{}
	if present {
		response.IOSShortcutInstallURL = shortcutURL
	}
	respondJSON(w, http.StatusOK, response)
}

func AdminUpdateIOSShortcutInstallURLHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	request := updateIOSShortcutInstallURLRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.IOSShortcutInstallURL == nil {
		http.Error(w, "ios_shortcut_install_url is required", http.StatusBadRequest)
		return
	}

	shortcutURL, present, err := SetIOSShortcutInstallURL(*request.IOSShortcutInstallURL)
	if err != nil {
		if err == ErrInvalidIOSShortcutInstallURL {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := iosShortcutInstallURLResponse{}
	if present {
		response.IOSShortcutInstallURL = shortcutURL
	}
	respondJSON(w, http.StatusOK, response)
}
