package service

import "strings"

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
		Label: "Open-Meteo",
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
	return target
}
