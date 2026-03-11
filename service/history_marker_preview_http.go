package service

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
)

const historyMarkerPreviewProfileList = "history-list"

var historyMarkerPreviewCurrentUserFn = currentUserFromRequest
var historyMarkerPreviewCurrentRelationFn = GetCurrentRelation
var historyMarkerPreviewGetMarkerFn = func(markerID uint) (*dbmodel.Marker, error) {
	var marker dbmodel.Marker
	marker.ID = markerID
	if err := marker.GetById(database.Connection); err != nil {
		return nil, err
	}
	return &marker, nil
}
var historyMarkerPreviewResolvePinFn = ResolveUserPreviewPinByUsername
var historyMarkerPreviewGenerateFn = GenerateStaticMapPreviewByUsername
var historyMarkerPreviewStorageDirFn = func() string {
	return filepath.Join(constant.BasePath, strings.TrimPrefix(constant.PreviewImagePath, "/"))
}

type historyMarkerPreviewResponse struct {
	MarkerID       uint   `json:"marker_id"`
	Profile        string `json:"profile"`
	State          string `json:"state"`
	CacheHit       bool   `json:"cache_hit"`
	ImageURL       string `json:"image_url,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

func buildHistoryMarkerPreviewKey(marker dbmodel.Marker, profile string, previewPin *dbmodel.Pin) string {
	parts := []string{
		strings.TrimSpace(profile),
		fmt.Sprintf("%.5f", marker.Latitude),
		fmt.Sprintf("%.5f", marker.Longitude),
		strings.ToLower(strings.TrimSpace(marker.Type)),
	}
	if previewPin != nil {
		parts = append(parts, strconv.FormatUint(uint64(previewPin.ID), 10))
		parts = append(parts, strings.TrimSpace(previewPin.ImagePath))
	}

	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func hasValidHistoryMarkerCoordinates(marker *dbmodel.Marker) bool {
	if marker == nil {
		return false
	}
	return validateCoordinates(marker.Latitude, marker.Longitude) == nil
}

func normalizeHistoryMarkerPreviewProfile(raw string) (string, error) {
	profile := strings.TrimSpace(strings.ToLower(raw))
	if profile == "" {
		return historyMarkerPreviewProfileList, nil
	}
	if profile != historyMarkerPreviewProfileList {
		return "", fmt.Errorf("unsupported history marker preview profile")
	}
	return profile, nil
}

func buildHistoryMarkerPreviewFallback(markerID uint, profile string, reason string) historyMarkerPreviewResponse {
	return historyMarkerPreviewResponse{
		MarkerID:       markerID,
		Profile:        profile,
		State:          "fallback",
		CacheHit:       false,
		Width:          defaultStaticPreviewWidth,
		Height:         defaultStaticPreviewHeight,
		FallbackReason: strings.TrimSpace(reason),
	}
}

func HistoryMarkerPreviewHandler(w http.ResponseWriter, r *http.Request) {
	user := historyMarkerPreviewCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	relation, err := historyMarkerPreviewCurrentRelationFn(*user)
	if relation == nil {
		if err == nil {
			http.Error(w, "cannot perform this without relation", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	markerID, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("marker_id")))
	if err != nil || markerID <= 0 {
		http.Error(w, "marker_id must be a positive integer", http.StatusBadRequest)
		return
	}

	profile, err := normalizeHistoryMarkerPreviewProfile(r.URL.Query().Get("profile"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	marker, err := historyMarkerPreviewGetMarkerFn(uint(markerID))
	if err != nil {
		http.Error(w, helper.CheckDatabaseError(err, &helper.MarkerNotFound{}).Error(), http.StatusNotFound)
		return
	}
	if marker.RelationId != relation.ID {
		http.Error(w, (&helper.InvalidRelationUpdateError{}).Error(), http.StatusForbidden)
		return
	}
	if !hasValidHistoryMarkerCoordinates(marker) {
		http.Error(w, "history marker is missing valid coordinates", http.StatusBadRequest)
		return
	}

	_, pin, err := historyMarkerPreviewResolvePinFn(user.Username)
	if err != nil {
		switch {
		case errors.Is(err, ErrPreviewPinSelectionRequired):
			respondJSON(w, http.StatusOK, buildHistoryMarkerPreviewFallback(marker.ID, profile, "preview_pin_not_configured"))
		case errors.Is(err, ErrPreviewPinInvalid):
			respondJSON(w, http.StatusOK, buildHistoryMarkerPreviewFallback(marker.ID, profile, "invalid_preview_pin"))
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	cacheKey := buildHistoryMarkerPreviewKey(*marker, profile, pin)
	fileName := fmt.Sprintf("history-marker-%s.png", cacheKey)
	relativePath := constant.PreviewImagePath + fileName
	storageDir := historyMarkerPreviewStorageDirFn()
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	absolutePath := filepath.Join(storageDir, fileName)
	if info, statErr := os.Stat(absolutePath); statErr == nil && info.Size() > 0 {
		log.Printf("history_marker_preview cache_hit marker_id=%d profile=%s relation_id=%d", marker.ID, profile, relation.ID)
		respondJSON(w, http.StatusOK, historyMarkerPreviewResponse{
			MarkerID: marker.ID,
			Profile:  profile,
			State:    "ready",
			CacheHit: true,
			ImageURL: relativePath,
			Width:    defaultStaticPreviewWidth,
			Height:   defaultStaticPreviewHeight,
		})
		return
	}

	result, err := historyMarkerPreviewGenerateFn(user.Username, marker.Type, marker.Latitude, marker.Longitude)
	if err != nil {
		fallbackReason := "preview_generation_failed"
		switch {
		case errors.Is(err, ErrPreviewPinSelectionRequired):
			fallbackReason = "preview_pin_not_configured"
		case errors.Is(err, ErrPreviewPinInvalid):
			fallbackReason = "invalid_preview_pin"
		case errors.Is(err, ErrTomTomStaticMap):
			fallbackReason = "tomtom_dependency_failure"
		case errors.Is(err, ErrImageComposition):
			fallbackReason = "image_composition_failure"
		}
		log.Printf("history_marker_preview fallback marker_id=%d profile=%s relation_id=%d reason=%s err=%v", marker.ID, profile, relation.ID, fallbackReason, err)
		respondJSON(w, http.StatusOK, buildHistoryMarkerPreviewFallback(marker.ID, profile, fallbackReason))
		return
	}

	if writeErr := os.WriteFile(absolutePath, result.Image, 0o644); writeErr != nil {
		http.Error(w, writeErr.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("history_marker_preview generated marker_id=%d profile=%s relation_id=%d cache_hit=false", marker.ID, profile, relation.ID)
	respondJSON(w, http.StatusOK, historyMarkerPreviewResponse{
		MarkerID: marker.ID,
		Profile:  profile,
		State:    "ready",
		CacheHit: false,
		ImageURL: relativePath,
		Width:    result.Width,
		Height:   result.Height,
	})
}
