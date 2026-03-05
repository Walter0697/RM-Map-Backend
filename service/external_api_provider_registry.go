package service

import (
	"strings"

	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
)

const (
	ExternalAPIProviderTomTomMap = "tomtom_map"
	ExternalAPIProviderMovieDB   = "movie_db"
	ExternalAPIProviderOpenMeteo = "open_meteo"
	ExternalAPIProviderWeatherUI = "weather_overlay_client"
)

type ExternalAPIProviderMetadata struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var managedExternalAPIProviders = []ExternalAPIProviderMetadata{
	{
		ID:    ExternalAPIProviderTomTomMap,
		Label: "TomTom",
	},
	{
		ID:    ExternalAPIProviderMovieDB,
		Label: "Movie DB",
	},
	{
		ID:    ExternalAPIProviderOpenMeteo,
		Label: "Open Meteo",
	},
	{
		ID:    ExternalAPIProviderWeatherUI,
		Label: "Weather Overlay Client",
	},
}

func ListManagedExternalAPIProviders() []ExternalAPIProviderMetadata {
	result := make([]ExternalAPIProviderMetadata, 0, len(managedExternalAPIProviders))
	for _, item := range managedExternalAPIProviders {
		result = append(result, item)
	}
	return result
}

func ListAvailableExternalAPIProviders() []ExternalAPIProviderMetadata {
	managed := ListManagedExternalAPIProviders()
	result := make([]ExternalAPIProviderMetadata, 0, len(managed))
	seen := make(map[string]bool, len(managed))
	for _, item := range managed {
		normalized := strings.TrimSpace(item.ID)
		if normalized == "" {
			continue
		}
		seen[normalized] = true
		result = append(result, item)
	}

	discovered := make([]string, 0)
	if err := database.Connection.Model(&dbmodel.ExternalAPIAuditEvent{}).
		Distinct("provider").
		Where("provider IS NOT NULL AND provider <> ''").
		Order("provider asc").
		Pluck("provider", &discovered).Error; err == nil {
		for _, provider := range discovered {
			normalized := strings.TrimSpace(provider)
			if normalized == "" || seen[normalized] {
				continue
			}
			seen[normalized] = true
			result = append(result, ExternalAPIProviderMetadata{
				ID:    normalized,
				Label: ExternalAPIProviderLabel(normalized),
			})
		}
	}

	return result
}

func IsManagedExternalAPIProvider(provider string) bool {
	target := strings.TrimSpace(provider)
	if target == "" {
		return false
	}
	for _, item := range managedExternalAPIProviders {
		if item.ID == target {
			return true
		}
	}
	return false
}

func ExternalAPIProviderLabel(provider string) string {
	target := strings.TrimSpace(provider)
	for _, item := range managedExternalAPIProviders {
		if item.ID == target {
			return item.Label
		}
	}
	if target == "" {
		return target
	}
	words := strings.Split(strings.ReplaceAll(target, "-", "_"), "_")
	for index, word := range words {
		if word == "" {
			continue
		}
		lower := strings.ToLower(word)
		words[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(words, " ")
}
