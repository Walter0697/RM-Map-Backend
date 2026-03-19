package constant

const (
	APIKeyScopeMarkersRead    = "markers:read"
	APIKeyScopeMarkersWrite   = "markers:write"
	APIKeyScopeSchedulesRead  = "schedules:read"
	APIKeyScopeSchedulesWrite = "schedules:write"
	APIKeyScopeStationsRead   = "stations:read"
	APIKeyScopeStationsWrite  = "stations:write"
	APIKeyScopeSettingsRead   = "settings:read"
	APIKeyScopeSettingsWrite  = "settings:write"
	APIKeyScopeStaticPreview  = "static-preview:generate"
	APIKeyScopeCalendarSync   = "calendar:sync"
	APIKeyScopeWeatherChatbot = "weather:chatbot"
)

func AllAPIKeyScopes() []string {
	return []string{
		APIKeyScopeMarkersRead,
		APIKeyScopeMarkersWrite,
		APIKeyScopeSchedulesRead,
		APIKeyScopeSchedulesWrite,
		APIKeyScopeStationsRead,
		APIKeyScopeStationsWrite,
		APIKeyScopeSettingsRead,
		APIKeyScopeSettingsWrite,
		APIKeyScopeStaticPreview,
		APIKeyScopeCalendarSync,
		APIKeyScopeWeatherChatbot,
	}
}
