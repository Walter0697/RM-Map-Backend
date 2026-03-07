package service

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
)

type weatherOverlayClientEventRequest struct {
	Event     string `json:"event"`
	Message   string `json:"message"`
	UserAgent string `json:"user_agent"`
}

func PlanningWeatherHandler(w http.ResponseWriter, r *http.Request) {
	if !WeatherPlanningEnabled() {
		respondJSON(w, http.StatusOK, WeatherPlanningResponse{
			Items:        []WeatherOverlayPoint{},
			Provider:     strings.TrimSpace(config.Data.Weather.Provider),
			GeneratedAt:  time.Now().UTC(),
			ErrorCode:    "feature_disabled",
			ErrorMessage: "weather overlay feature is disabled",
			Freshness: WeatherFreshnessMeta{
				Status:   weatherFreshnessUnavailable,
				Stale:    true,
				Degraded: true,
				Source:   weatherSourceDegraded,
			},
		})
		return
	}

	input, err := parsePlanningWeatherInput(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := GetPlanningWeather(input)
	rainCount, snowCount, noneCount := weatherOverlayTypeCounts(response.Items)
	log.Printf(
		"weather planning response provider=%s source=%s freshness=%s items=%d rain=%d snow=%d none=%d error_code=%s viewport=[%.5f,%.5f,%.5f,%.5f] center=[%.5f,%.5f] zoom=%.2f forecast_h=%d day_offset=%d",
		response.Provider,
		response.Freshness.Source,
		response.Freshness.Status,
		len(response.Items),
		rainCount,
		snowCount,
		noneCount,
		response.ErrorCode,
		input.MinLat,
		input.MaxLat,
		input.MinLon,
		input.MaxLon,
		input.CenterLat,
		input.CenterLon,
		input.Zoom,
		input.ForecastWindowH,
		input.ForecastDayOffset,
	)
	respondJSON(w, http.StatusOK, response)
}

func WeatherOverlayClientEventHandler(w http.ResponseWriter, r *http.Request) {
	payload, err := readJSONBody(r)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	request := weatherOverlayClientEventRequest{}
	if err := decodeStrictJSONPayload(payload, &request); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	status := dbmodel.ExternalAPIAuditStatusSuccess
	if strings.TrimSpace(request.Event) == "" {
		status = dbmodel.ExternalAPIAuditStatusError
		request.Event = "unknown_event"
	}
	_ = createExternalAPIAuditEventFn(ExternalAPIAuditWriteInput{
		Provider:    ExternalAPIProviderWeatherUI,
		Operation:   "overlay_" + strings.TrimSpace(request.Event),
		StatusClass: status,
		LatencyMS:   0,
		RequestTime: now,
		ErrorClass:  "client_render",
		ErrorDetail: strings.TrimSpace(request.Message),
		CompletedAt: &now,
	})

	respondJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func parsePlanningWeatherInput(r *http.Request) (WeatherPlanningInput, error) {
	queries := r.URL.Query()
	minLat, err := parseFloatQuery(queries.Get("min_lat"), "min_lat")
	if err != nil {
		return WeatherPlanningInput{}, err
	}
	maxLat, err := parseFloatQuery(queries.Get("max_lat"), "max_lat")
	if err != nil {
		return WeatherPlanningInput{}, err
	}
	minLon, err := parseFloatQuery(queries.Get("min_lon"), "min_lon")
	if err != nil {
		return WeatherPlanningInput{}, err
	}
	maxLon, err := parseFloatQuery(queries.Get("max_lon"), "max_lon")
	if err != nil {
		return WeatherPlanningInput{}, err
	}
	centerLat, err := parseFloatQuery(queries.Get("center_lat"), "center_lat")
	if err != nil {
		return WeatherPlanningInput{}, err
	}
	centerLon, err := parseFloatQuery(queries.Get("center_lon"), "center_lon")
	if err != nil {
		return WeatherPlanningInput{}, err
	}
	zoom, err := parseFloatQuery(queries.Get("zoom"), "zoom")
	if err != nil {
		return WeatherPlanningInput{}, err
	}
	forecastWindowH := weatherMaxForecastHours()
	forecastDayOffset := 0
	rawWindow := strings.TrimSpace(queries.Get("forecast_window_h"))
	if rawWindow != "" {
		parsedWindow, convErr := strconv.Atoi(rawWindow)
		if convErr != nil {
			return WeatherPlanningInput{}, convErr
		}
		forecastWindowH = parsedWindow
	}
	rawOffset := strings.TrimSpace(queries.Get("forecast_day_offset"))
	if rawOffset != "" {
		parsedOffset, convErr := strconv.Atoi(rawOffset)
		if convErr != nil {
			return WeatherPlanningInput{}, convErr
		}
		forecastDayOffset = parsedOffset
	}

	return WeatherPlanningInput{
		MinLat:            minLat,
		MaxLat:            maxLat,
		MinLon:            minLon,
		MaxLon:            maxLon,
		CenterLat:         centerLat,
		CenterLon:         centerLon,
		Zoom:              zoom,
		ForecastWindowH:   forecastWindowH,
		ForecastDayOffset: forecastDayOffset,
	}, nil
}

func parseFloatQuery(value string, name string) (float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, strconv.ErrSyntax
	}
	output, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, err
	}
	return output, nil
}

func weatherOverlayTypeCounts(items []WeatherOverlayPoint) (int, int, int) {
	rainCount := 0
	snowCount := 0
	noneCount := 0
	for _, item := range items {
		switch strings.TrimSpace(item.PrecipitationType) {
		case "rain":
			rainCount++
		case "snow":
			snowCount++
		default:
			noneCount++
		}
	}
	return rainCount, snowCount, noneCount
}
