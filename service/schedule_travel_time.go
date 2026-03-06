package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	scheduleTravelStatusOK          = "ok"
	scheduleTravelStatusUnavailable = "unavailable"

	scheduleTravelDifficultyEasy        = "easy"
	scheduleTravelDifficultyModerate    = "moderate"
	scheduleTravelDifficultyDifficult   = "difficult"
	scheduleTravelDifficultyUnavailable = "unavailable"

	defaultTomTomRoutingBaseURL = "https://api.tomtom.com/routing/1"
	defaultScheduleTravelMS     = 3000
)

type ScheduleTravelPoint struct {
	ScheduleID int      `json:"schedule_id"`
	MarkerID   *int     `json:"marker_id,omitempty"`
	Label      string   `json:"label,omitempty"`
	Latitude   *float64 `json:"lat,omitempty"`
	Longitude  *float64 `json:"lon,omitempty"`
	SelectedAt *string  `json:"selected_date,omitempty"`
}

type ScheduleTravelTransitionEndpoint struct {
	ScheduleID int      `json:"schedule_id"`
	MarkerID   *int     `json:"marker_id,omitempty"`
	Label      string   `json:"label,omitempty"`
	Latitude   *float64 `json:"lat,omitempty"`
	Longitude  *float64 `json:"lon,omitempty"`
}

type ScheduleTravelTransitionAnalysis struct {
	Origin         ScheduleTravelTransitionEndpoint `json:"origin"`
	Destination    ScheduleTravelTransitionEndpoint `json:"destination"`
	DurationSecond *int                             `json:"duration_seconds,omitempty"`
	ScheduledGap   *int                             `json:"scheduled_gap_seconds,omitempty"`
	DeltaSecond    *int                             `json:"delta_seconds,omitempty"`
	Difficulty     string                           `json:"difficulty"`
	Status         string                           `json:"status"`
}

type tomTomRouteResponse struct {
	Routes []struct {
		Summary struct {
			TravelTimeInSeconds int `json:"travelTimeInSeconds"`
		} `json:"summary"`
	} `json:"routes"`
}

type scheduleTravelDurationLookup func(originLat float64, originLon float64, destinationLat float64, destinationLon float64) (int, error)

var integrationTravelDurationLookupFn scheduleTravelDurationLookup = fetchTomTomTravelDurationSeconds
var scheduleTravelThresholdsProviderFn = GetScheduleTravelThresholds

func BuildScheduleTransitionAnalysis(points []ScheduleTravelPoint) []ScheduleTravelTransitionAnalysis {
	if len(points) <= 1 {
		return []ScheduleTravelTransitionAnalysis{}
	}

	thresholds, thresholdErr := scheduleTravelThresholdsProviderFn()
	if thresholdErr != nil {
		log.Printf("[schedule-travel] threshold_resolve_error=%v fallback_to_defaults=true", thresholdErr)
		thresholds = ScheduleTravelThresholds{
			EasyThresholdMinutes:      20,
			DifficultThresholdMinutes: 45,
		}
	}

	output := make([]ScheduleTravelTransitionAnalysis, 0, len(points)-1)
	for index := 0; index < len(points)-1; index++ {
		origin := points[index]
		destination := points[index+1]

		result := ScheduleTravelTransitionAnalysis{
			Origin: ScheduleTravelTransitionEndpoint{
				ScheduleID: origin.ScheduleID,
				MarkerID:   origin.MarkerID,
				Label:      origin.Label,
				Latitude:   origin.Latitude,
				Longitude:  origin.Longitude,
			},
			Destination: ScheduleTravelTransitionEndpoint{
				ScheduleID: destination.ScheduleID,
				MarkerID:   destination.MarkerID,
				Label:      destination.Label,
				Latitude:   destination.Latitude,
				Longitude:  destination.Longitude,
			},
			Difficulty: scheduleTravelDifficultyUnavailable,
			Status:     scheduleTravelStatusUnavailable,
		}

		if origin.Latitude == nil || origin.Longitude == nil || destination.Latitude == nil || destination.Longitude == nil {
			output = append(output, result)
			continue
		}

		start := time.Now()
		durationSeconds, err := integrationTravelDurationLookupFn(*origin.Latitude, *origin.Longitude, *destination.Latitude, *destination.Longitude)
		latencyMS := time.Since(start).Milliseconds()
		if err != nil {
			log.Printf("[schedule-travel] status=unavailable origin_schedule_id=%d destination_schedule_id=%d latency_ms=%d error=%v",
				origin.ScheduleID, destination.ScheduleID, latencyMS, err)
			output = append(output, result)
			continue
		}

		result.DurationSecond = &durationSeconds
		gapSeconds, hasGap := scheduleGapSeconds(origin.SelectedAt, destination.SelectedAt)
		if hasGap {
			result.ScheduledGap = &gapSeconds
			deltaSeconds := gapSeconds - durationSeconds
			result.DeltaSecond = &deltaSeconds
			result.Difficulty = classifyTravelDifficultyByDelta(deltaSeconds, thresholds)
			result.Status = scheduleTravelStatusOK
		} else {
			result.Difficulty = scheduleTravelDifficultyUnavailable
			result.Status = scheduleTravelStatusUnavailable
		}
		log.Printf("[schedule-travel] status=%s origin_schedule_id=%d destination_schedule_id=%d latency_ms=%d duration_seconds=%d difficulty=%s",
			result.Status, origin.ScheduleID, destination.ScheduleID, latencyMS, durationSeconds, result.Difficulty)
		output = append(output, result)
	}
	return output
}

func classifyTravelDifficultyByDelta(deltaSeconds int, thresholds ScheduleTravelThresholds) string {
	deltaMinutes := float64(deltaSeconds) / 60.0
	if deltaMinutes < 0 {
		return scheduleTravelDifficultyDifficult
	}
	if deltaMinutes < float64(thresholds.EasyThresholdMinutes) {
		return scheduleTravelDifficultyModerate
	}
	return scheduleTravelDifficultyEasy
}

func fetchTomTomTravelDurationSeconds(originLat float64, originLon float64, destinationLat float64, destinationLon float64) (int, error) {
	apiKey := strings.TrimSpace(config.Data.APIKEY.TomTomMap)
	requestStarted := time.Now().UTC()
	if apiKey == "" {
		now := time.Now().UTC()
		err := fmt.Errorf("tomtom api key is not configured")
		RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    ExternalAPIProviderTomTomMap,
			Operation:   "calculate_route",
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			LatencyMS:   now.Sub(requestStarted).Milliseconds(),
			RequestTime: requestStarted,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return 0, err
	}

	baseURL := strings.TrimSpace(config.Data.ScheduleTravel.BaseURL)
	if baseURL == "" {
		baseURL = defaultTomTomRoutingBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeoutMS := config.Data.ScheduleTravel.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = defaultScheduleTravelMS
	}

	path := fmt.Sprintf("/calculateRoute/%f,%f:%f,%f/json", originLat, originLon, destinationLat, destinationLon)
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return 0, err
	}
	query := u.Query()
	query.Set("key", apiKey)
	query.Set("traffic", "true")
	query.Set("travelMode", "car")
	u.RawQuery = query.Encode()

	client := &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		now := time.Now().UTC()
		RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    ExternalAPIProviderTomTomMap,
			Operation:   "calculate_route",
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			LatencyMS:   now.Sub(requestStarted).Milliseconds(),
			RequestTime: requestStarted,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return 0, err
	}

	response, err := client.Do(request)
	if err != nil {
		now := time.Now().UTC()
		RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    ExternalAPIProviderTomTomMap,
			Operation:   "calculate_route",
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			LatencyMS:   now.Sub(requestStarted).Milliseconds(),
			RequestTime: requestStarted,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return 0, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	now := time.Now().UTC()
	statusCode := response.StatusCode
	if err != nil {
		RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    ExternalAPIProviderTomTomMap,
			Operation:   "calculate_route",
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			HTTPStatus:  &statusCode,
			LatencyMS:   now.Sub(requestStarted).Milliseconds(),
			RequestTime: requestStarted,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return 0, err
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		httpErr := fmt.Errorf("status code %d", response.StatusCode)
		RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    ExternalAPIProviderTomTomMap,
			Operation:   "calculate_route",
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			HTTPStatus:  &statusCode,
			LatencyMS:   now.Sub(requestStarted).Milliseconds(),
			RequestTime: requestStarted,
			ErrorClass:  classifyExternalAPIErr(httpErr),
			ErrorDetail: httpErr.Error(),
			CompletedAt: &now,
		})
		return 0, httpErr
	}

	result := tomTomRouteResponse{}
	if err := json.Unmarshal(body, &result); err != nil {
		RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    ExternalAPIProviderTomTomMap,
			Operation:   "calculate_route",
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			HTTPStatus:  &statusCode,
			LatencyMS:   now.Sub(requestStarted).Milliseconds(),
			RequestTime: requestStarted,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return 0, err
	}
	if len(result.Routes) == 0 || result.Routes[0].Summary.TravelTimeInSeconds <= 0 {
		err := fmt.Errorf("no route result")
		RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    ExternalAPIProviderTomTomMap,
			Operation:   "calculate_route",
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			HTTPStatus:  &statusCode,
			LatencyMS:   now.Sub(requestStarted).Milliseconds(),
			RequestTime: requestStarted,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return 0, err
	}

	RecordExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
		Provider:    ExternalAPIProviderTomTomMap,
		Operation:   "calculate_route",
		StatusClass: dbmodel.ExternalAPIAuditStatusSuccess,
		HTTPStatus:  &statusCode,
		LatencyMS:   now.Sub(requestStarted).Milliseconds(),
		RequestTime: requestStarted,
		CompletedAt: &now,
	})

	return result.Routes[0].Summary.TravelTimeInSeconds, nil
}

func BuildScheduleTravelPointsFromSchedules(schedules []dbmodel.Schedule) []ScheduleTravelPoint {
	points := make([]ScheduleTravelPoint, 0, len(schedules))
	for _, schedule := range schedules {
		selectedAt := schedule.SelectedDate.Format(time.RFC3339)
		point := ScheduleTravelPoint{
			ScheduleID: int(schedule.ID),
			Label:      schedule.Label,
			SelectedAt: &selectedAt,
		}
		if schedule.MarkerId != nil {
			markerID := int(*schedule.MarkerId)
			point.MarkerID = &markerID
		}
		if schedule.SelectedMarker != nil {
			lat := schedule.SelectedMarker.Latitude
			lon := schedule.SelectedMarker.Longitude
			point.Latitude = &lat
			point.Longitude = &lon
		}
		points = append(points, point)
	}
	return points
}

func scheduleGapSeconds(originSelectedAt *string, destinationSelectedAt *string) (int, bool) {
	if originSelectedAt == nil || destinationSelectedAt == nil {
		return 0, false
	}
	originTime, originErr := time.Parse(time.RFC3339, strings.TrimSpace(*originSelectedAt))
	if originErr != nil {
		return 0, false
	}
	destinationTime, destinationErr := time.Parse(time.RFC3339, strings.TrimSpace(*destinationSelectedAt))
	if destinationErr != nil {
		return 0, false
	}
	return int(destinationTime.Sub(originTime).Seconds()), true
}

func parseScheduleTravelTimeoutMS(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}
	return value, nil
}
