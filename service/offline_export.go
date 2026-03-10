package service

import (
	"fmt"
	"mapmarker/backend/database/dbmodel"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	ExportSnapshotSchemaVersion = "v1"

	offlineExportWebsiteStatusAvailable = "available"
	offlineExportWebsiteStatusFallback  = "fallback"
	offlineExportWebsiteStatusMissing   = "missing"

	offlineExportEstimatedTimeSourceMarker = "marker_estimate"
	offlineExportEstimatedTimeSourceNone   = "none"
)

type ExportSnapshot struct {
	SchemaVersion string                   `json:"schema_version"`
	GeneratedAt   time.Time                `json:"generated_at"`
	Timezone      string                   `json:"timezone"`
	Source        ExportSnapshotSource     `json:"source"`
	Markers       []ExportSnapshotMarker   `json:"markers"`
	Schedules     []ExportSnapshotSchedule `json:"schedules"`
}

type ExportSnapshotSource struct {
	RelationID uint `json:"relation_id"`
	UserID     uint `json:"user_id,omitempty"`
}

type ExportSnapshotMarker struct {
	ID           uint                        `json:"id"`
	Label        string                      `json:"label"`
	Latitude     float64                     `json:"latitude"`
	Longitude    float64                     `json:"longitude"`
	Address      string                      `json:"address,omitempty"`
	Country      string                      `json:"country,omitempty"`
	CountryCode  string                      `json:"country_code,omitempty"`
	CountryPart  string                      `json:"country_part,omitempty"`
	Description  string                      `json:"description,omitempty"`
	Website      ExportSnapshotWebsite       `json:"website"`
	EstimateTime ExportSnapshotEstimatedTime `json:"estimate_time"`
}

type ExportSnapshotWebsite struct {
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id,omitempty"`
	URL        string `json:"url,omitempty"`
	Status     string `json:"status"`
}

type ExportSnapshotEstimatedTime struct {
	Raw     string `json:"raw,omitempty"`
	Display string `json:"display"`
	Source  string `json:"source"`
	Missing bool   `json:"missing"`
}

type ExportSnapshotSchedule struct {
	ID           uint                        `json:"id"`
	Label        string                      `json:"label"`
	Description  string                      `json:"description,omitempty"`
	Status       string                      `json:"status"`
	SelectedDate time.Time                   `json:"selected_date"`
	MarkerID     *uint                       `json:"marker_id,omitempty"`
	EstimateTime ExportSnapshotEstimatedTime `json:"estimate_time"`
}

type BuildExportSnapshotInput struct {
	RelationID uint
	UserID     uint
	Timezone   string
	Now        time.Time
	Markers    []dbmodel.Marker
	Schedules  []dbmodel.Schedule
}

func BuildExportSnapshot(input BuildExportSnapshotInput) (*ExportSnapshot, error) {
	tz := strings.TrimSpace(input.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, fmt.Errorf("invalid timezone: %s", tz)
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	generatedAt := now.UTC()

	markers := make([]dbmodel.Marker, 0, len(input.Markers))
	markers = append(markers, input.Markers...)
	sort.SliceStable(markers, func(i int, j int) bool {
		return markers[i].ID < markers[j].ID
	})

	schedules := make([]dbmodel.Schedule, 0, len(input.Schedules))
	schedules = append(schedules, input.Schedules...)
	sort.SliceStable(schedules, func(i int, j int) bool {
		return schedules[i].ID < schedules[j].ID
	})

	result := &ExportSnapshot{
		SchemaVersion: ExportSnapshotSchemaVersion,
		GeneratedAt:   generatedAt,
		Timezone:      tz,
		Source: ExportSnapshotSource{
			RelationID: input.RelationID,
			UserID:     input.UserID,
		},
		Markers:   make([]ExportSnapshotMarker, 0, len(markers)),
		Schedules: make([]ExportSnapshotSchedule, 0, len(schedules)),
	}

	for _, marker := range markers {
		result.Markers = append(result.Markers, ExportSnapshotMarker{
			ID:           marker.ID,
			Label:        marker.Label,
			Latitude:     marker.Latitude,
			Longitude:    marker.Longitude,
			Address:      marker.Address,
			Country:      marker.Country,
			CountryCode:  marker.CountryCode,
			CountryPart:  marker.CountryPart,
			Description:  marker.Description,
			Website:      projectMarkerWebsite(marker),
			EstimateTime: normalizeEstimatedTime(marker.EstimateTime),
		})
	}

	for _, schedule := range schedules {
		estimateTime := normalizeEstimatedTime("")
		if schedule.SelectedMarker != nil {
			estimateTime = normalizeEstimatedTime(schedule.SelectedMarker.EstimateTime)
		}

		result.Schedules = append(result.Schedules, ExportSnapshotSchedule{
			ID:           schedule.ID,
			Label:        schedule.Label,
			Description:  schedule.Description,
			Status:       schedule.Status,
			SelectedDate: schedule.SelectedDate,
			MarkerID:     schedule.MarkerId,
			EstimateTime: estimateTime,
		})
	}

	return result, nil
}

func normalizeEstimatedTime(value string) ExportSnapshotEstimatedTime {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ExportSnapshotEstimatedTime{
			Display: "N/A",
			Source:  offlineExportEstimatedTimeSourceNone,
			Missing: true,
		}
	}
	return ExportSnapshotEstimatedTime{
		Raw:     trimmed,
		Display: trimmed,
		Source:  offlineExportEstimatedTimeSourceMarker,
		Missing: false,
	}
}

func projectMarkerWebsite(marker dbmodel.Marker) ExportSnapshotWebsite {
	if marker.RestaurantInfo != nil {
		provider := NormalizeMarkerWebsiteProviderID(marker.RestaurantInfo.Source)
		if provider == "" {
			provider = "unknown"
		}
		providerID := strings.TrimSpace(marker.RestaurantInfo.SourceId)
		websiteURL := sanitizeWebsiteURL(marker.RestaurantInfo.Website)
		if websiteURL != "" {
			return ExportSnapshotWebsite{
				Provider:   provider,
				ProviderID: providerID,
				URL:        websiteURL,
				Status:     offlineExportWebsiteStatusAvailable,
			}
		}

		fallbackURL := sanitizeWebsiteURL(marker.Link)
		if fallbackURL != "" {
			return ExportSnapshotWebsite{
				Provider:   provider,
				ProviderID: providerID,
				URL:        fallbackURL,
				Status:     offlineExportWebsiteStatusFallback,
			}
		}

		return ExportSnapshotWebsite{
			Provider:   provider,
			ProviderID: providerID,
			Status:     offlineExportWebsiteStatusMissing,
		}
	}

	fallbackURL := sanitizeWebsiteURL(marker.Link)
	if fallbackURL != "" {
		return ExportSnapshotWebsite{
			Provider: "unknown",
			URL:      fallbackURL,
			Status:   offlineExportWebsiteStatusFallback,
		}
	}

	return ExportSnapshotWebsite{
		Provider: "unknown",
		Status:   offlineExportWebsiteStatusMissing,
	}
}

func sanitizeWebsiteURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return ""
	}
	return parsed.String()
}
