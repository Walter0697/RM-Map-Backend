package service

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"
)

const (
	chatbotWeatherDefaultRadius = 0.03
)

var (
	chatbotWeatherPlanningFn      = GetPlanningWeather
	chatbotWeatherStationLookupFn = defaultChatbotStationLookup
)

type ChatbotWeatherResponse struct {
	ServedAt  time.Time              `json:"served_at"`
	Location  ChatbotWeatherLocation `json:"location"`
	Forecast  ChatbotWeatherForecast `json:"forecast"`
	Freshness WeatherFreshnessMeta   `json:"freshness"`
	Warnings  []string               `json:"warnings"`
}

type ChatbotWeatherLocation struct {
	Latitude  float64                `json:"latitude"`
	Longitude float64                `json:"longitude"`
	City      string                 `json:"city,omitempty"`
	Station   *ChatbotWeatherStation `json:"station,omitempty"`
}

type ChatbotWeatherStation struct {
	MapName    string `json:"map_name"`
	Identifier string `json:"identifier"`
	Label      string `json:"label,omitempty"`
}

type ChatbotWeatherForecast struct {
	Summary             string   `json:"summary"`
	Confidence          string   `json:"confidence"`
	PrecipitationChance float64  `json:"precipitation_chance"`
	Temperature         *float64 `json:"temperature,omitempty"`
	TemperatureUnit     string   `json:"temperature_unit,omitempty"`
	Horizon             string   `json:"horizon"`
}

type chatbotWeatherRequest struct {
	City              string
	StationMap        string
	StationIdentifier string
	Latitude          *float64
	Longitude         *float64
	ForecastWindowH   int
	ForecastDayOffset int
	Horizon           string
}

func ChatbotWeatherHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := authenticateIntegrationRequest(w, r, "integration.chatbot.weather", constant.APIKeyScopeWeatherChatbot, "")
	if !ok {
		return
	}

	limit := config.Data.Weather.RateLimitPerMinute
	now := time.Now().UTC()
	allowed := weatherLimiter.allow(now, limit)
	snapshot := weatherLimiter.snapshot(limit, now)
	setChatbotRateLimitHeaders(w, limit, snapshot)
	if !allowed {
		retry := snapshot.resetSeconds
		if retry <= 0 {
			retry = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		respondJSON(w, http.StatusTooManyRequests, map[string]string{
			"error":   "rate_limit_exceeded",
			"message": "Too many chatbot weather requests. Try again later.",
		})
		return
	}

	request, err := parseChatbotWeatherQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	location, err := resolveChatbotWeatherLocation(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	input := buildChatbotWeatherInput(location, request)
	response := chatbotWeatherPlanningFn(input)
	servedAt := time.Now().UTC()

	forecast := buildChatbotWeatherForecast(response, request.Horizon)
	warnings := buildChatbotWeatherWarnings(response)
	if len(warnings) > 0 {
		log.Printf("chatbot weather warnings location=%s horizon=%s warnings=%v error_code=%s", describeChatbotLocation(location), request.Horizon, warnings, response.ErrorCode)
	}
	if response.ErrorCode != "" {
		log.Printf("chatbot weather upstream error location=%s code=%s message=%s", describeChatbotLocation(location), response.ErrorCode, response.ErrorMessage)
	}

	statusCode := http.StatusOK
	if response.ErrorCode != "" && len(response.Items) == 0 {
		statusCode = http.StatusServiceUnavailable
	}

	respondJSON(w, statusCode, ChatbotWeatherResponse{
		ServedAt:  servedAt,
		Location:  location,
		Forecast:  forecast,
		Freshness: response.Freshness,
		Warnings:  warnings,
	})
}

func parseChatbotWeatherQuery(r *http.Request) (chatbotWeatherRequest, error) {
	queries := r.URL.Query()
	var req chatbotWeatherRequest
	req.City = strings.TrimSpace(queries.Get("city"))
	req.StationMap = strings.TrimSpace(queries.Get("station_map"))
	req.StationIdentifier = strings.TrimSpace(queries.Get("station_identifier"))
	var err error
	if req.Latitude, err = parseOptionalFloat(queries.Get("lat"), "lat"); err != nil {
		return req, err
	}
	if req.Longitude, err = parseOptionalFloat(queries.Get("lon"), "lon"); err != nil {
		return req, err
	}

	if windowRaw := strings.TrimSpace(queries.Get("forecast_window_h")); windowRaw != "" {
		window, parseErr := strconv.Atoi(windowRaw)
		if parseErr != nil {
			return req, fmt.Errorf("invalid forecast_window_h: %w", parseErr)
		}
		req.ForecastWindowH = window
	}

	if offsetRaw := strings.TrimSpace(queries.Get("forecast_day_offset")); offsetRaw != "" {
		offset, parseErr := strconv.Atoi(offsetRaw)
		if parseErr != nil {
			return req, fmt.Errorf("invalid forecast_day_offset: %w", parseErr)
		}
		if offset < 0 {
			return req, fmt.Errorf("forecast_day_offset cannot be negative")
		}
		req.ForecastDayOffset = offset
	}

	req.Horizon = normalizeHorizon(strings.TrimSpace(queries.Get("horizon")))
	if req.Horizon == "" {
		req.Horizon = "current"
	}

	return req, nil
}

func parseOptionalFloat(rawValue string, key string) (*float64, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return nil, nil
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	return &value, nil
}

func resolveChatbotWeatherLocation(req chatbotWeatherRequest) (ChatbotWeatherLocation, error) {
	location := ChatbotWeatherLocation{City: req.City}
	var latPtr, lonPtr *float64

	if req.StationIdentifier != "" {
		station, err := chatbotWeatherStationLookupFn(req.StationMap, req.StationIdentifier)
		if err != nil {
			if utils.RecordNotFound(err) {
				return location, fmt.Errorf("station identifier %s not found", req.StationIdentifier)
			}
			return location, err
		}
		latPtr = ptrFloat(station.MapX)
		lonPtr = ptrFloat(station.MapY)
		location.Station = &ChatbotWeatherStation{
			MapName:    station.MapName,
			Identifier: station.Identifier,
			Label:      station.Label,
		}
	}

	if req.Latitude != nil {
		latPtr = req.Latitude
	}
	if req.Longitude != nil {
		lonPtr = req.Longitude
	}

	if latPtr == nil || lonPtr == nil {
		return location, fmt.Errorf("station identifier or lat/lon is required")
	}

	location.Latitude = *latPtr
	location.Longitude = *lonPtr
	return location, nil
}

func buildChatbotWeatherInput(location ChatbotWeatherLocation, req chatbotWeatherRequest) WeatherPlanningInput {
	radius := chatbotWeatherDefaultRadius
	minLat := clamp(location.Latitude-radius, -90, 90)
	maxLat := clamp(location.Latitude+radius, -90, 90)
	minLon := clamp(location.Longitude-radius, -180, 180)
	maxLon := clamp(location.Longitude+radius, -180, 180)

	window := req.ForecastWindowH
	if window <= 0 {
		window = forecastWindowForHorizon(req.Horizon)
	}
	maxWindow := weatherMaxForecastHours()
	if maxWindow <= 0 {
		maxWindow = 1
	}
	if window > maxWindow {
		window = maxWindow
	}
	if window <= 0 {
		window = 1
	}

	offset := req.ForecastDayOffset
	if offset < 0 {
		offset = 0
	}

	return WeatherPlanningInput{
		MinLat:            minLat,
		MaxLat:            maxLat,
		MinLon:            minLon,
		MaxLon:            maxLon,
		CenterLat:         location.Latitude,
		CenterLon:         location.Longitude,
		Zoom:              12,
		ForecastWindowH:   window,
		ForecastDayOffset: offset,
	}
}

func buildChatbotWeatherForecast(response WeatherPlanningResponse, horizon string) ChatbotWeatherForecast {
	forecast := ChatbotWeatherForecast{
		Horizon:    describeHorizon(horizon),
		Confidence: deriveChatbotConfidence(response.Freshness.Status),
	}

	if len(response.Items) == 0 {
		forecast.Summary = fmt.Sprintf("No forecast data available for the next %s.", forecast.Horizon)
		return forecast
	}

	total := len(response.Items)
	precipCount := 0
	typeCounts := make(map[string]int)
	highestIntensity := "none"
	highestPriority := 0
	temperatureSum := 0.0
	temperatureCount := 0

	for _, item := range response.Items {
		itemType := strings.ToLower(strings.TrimSpace(item.PrecipitationType))
		itemIntensity := strings.ToLower(strings.TrimSpace(item.Intensity))
		if itemType != "none" && item.PrecipitationMM > 0 {
			precipCount++
			typeCounts[itemType]++
			priority := chatBotIntensityPriority(itemIntensity)
			if priority > highestPriority {
				highestPriority = priority
				highestIntensity = itemIntensity
			}
		}
		if item.Temperature != nil {
			temperatureSum += *item.Temperature
			temperatureCount++
		}
	}

	if highestIntensity == "" {
		highestIntensity = "none"
	}

	forecast.PrecipitationChance = float64(precipCount) / float64(total)
	dominantType := determineDominantPrecipType(typeCounts)
	if dominantType == "" {
		dominantType = "clear"
	}

	if forecast.PrecipitationChance == 0 {
		forecast.Summary = fmt.Sprintf("No precipitation expected over the next %s.", forecast.Horizon)
	} else {
		forecast.Summary = fmt.Sprintf("Expect %s %s over the next %s.", highestIntensity, dominantType, forecast.Horizon)
	}

	if temperatureCount > 0 {
		avgTemp := temperatureSum / float64(temperatureCount)
		forecast.Temperature = ptrFloat(avgTemp)
		if response.TemperatureUnit != "" {
			forecast.TemperatureUnit = response.TemperatureUnit
		} else {
			forecast.TemperatureUnit = "°C"
		}
	}

	return forecast
}

func buildChatbotWeatherWarnings(response WeatherPlanningResponse) []string {
	warnings := make([]string, 0, 2)
	if response.Freshness.Stale {
		warnings = append(warnings, fmt.Sprintf("data may be stale (age=%ds, source=%s)", response.Freshness.AgeSeconds, response.Freshness.Source))
	}
	if response.ErrorMessage != "" {
		warnings = append(warnings, fmt.Sprintf("upstream message: %s", response.ErrorMessage))
	}
	if response.ErrorCode != "" && len(response.Items) > 0 {
		warnings = append(warnings, fmt.Sprintf("error %s observed while serving cached data", response.ErrorCode))
	}
	return warnings
}

func describeChatbotLocation(location ChatbotWeatherLocation) string {
	if location.Station != nil {
		return fmt.Sprintf("station=%s map=%s", location.Station.Identifier, location.Station.MapName)
	}
	if location.City != "" {
		return fmt.Sprintf("city=%s lat=%.4f lon=%.4f", location.City, location.Latitude, location.Longitude)
	}
	return fmt.Sprintf("lat=%.4f lon=%.4f", location.Latitude, location.Longitude)
}

func describeHorizon(horizon string) string {
	switch strings.ToLower(horizon) {
	case "current":
		return "a few hours"
	case "hourly":
		return "the next 12 hours"
	case "daily":
		return "the next 24 hours"
	default:
		return "the next few hours"
	}
}

func deriveChatbotConfidence(status string) string {
	switch status {
	case weatherFreshnessFresh:
		return "high"
	case weatherFreshnessStale:
		return "medium"
	default:
		return "low"
	}
}

func chatBotIntensityPriority(value string) int {
	switch value {
	case "heavy":
		return 3
	case "moderate":
		return 2
	case "light":
		return 1
	default:
		return 0
	}
}

func determineDominantPrecipType(counts map[string]int) string {
	best := ""
	highest := 0
	for kind, value := range counts {
		if value > highest {
			highest = value
			best = kind
		}
	}
	return best
}

func forecastWindowForHorizon(horizon string) int {
	switch strings.ToLower(horizon) {
	case "current":
		return 3
	case "hourly":
		return 12
	case "daily":
		return 24
	default:
		return 6
	}
}

func normalizeHorizon(raw string) string {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	switch candidate {
	case "current", "hourly", "daily":
		return candidate
	default:
		return ""
	}
}

func setChatbotRateLimitHeaders(w http.ResponseWriter, limit int, snapshot rateLimitSnapshot) {
	if limit <= 0 {
		return
	}
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(snapshot.remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.Itoa(snapshot.resetSeconds))
}

func defaultChatbotStationLookup(mapName, identifier string) (*dbmodel.TrainStation, error) {
	trimmedIdentifier := strings.TrimSpace(identifier)
	if trimmedIdentifier == "" {
		return nil, fmt.Errorf("station identifier is required")
	}
	query := database.Connection.Model(&dbmodel.TrainStation{}).Where("identifier = ?", trimmedIdentifier)
	if trimmedMap := strings.TrimSpace(mapName); trimmedMap != "" {
		query = query.Where("map_name = ?", trimmedMap)
	}
	var station dbmodel.TrainStation
	if err := query.First(&station).Error; err != nil {
		return nil, err
	}
	return &station, nil
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
