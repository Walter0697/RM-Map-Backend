package service

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type scheduleRoutePreviewRequest struct {
	Origin      RouteCoordinate `json:"origin"`
	Destination RouteCoordinate `json:"destination"`
}

type scheduleRoutePreviewResponse struct {
	Route       *RoutePlanResult `json:"route"`
	ImageBase64 string           `json:"image_base64"`
	MimeType    string           `json:"mime_type"`
	Format      string           `json:"format"`
	Width       int              `json:"width"`
	Height      int              `json:"height"`
}

func ScheduleRoutePreviewHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	request := scheduleRoutePreviewRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeIntegrationError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
		return
	}

	routeResult, routeErr := PlanTomTomRouteWithETA(r.Context(), request.Origin, request.Destination)
	if routeErr != nil {
		if routeServiceErr, ok := routeErr.(*RouteServiceError); ok {
			log.Printf("[schedule-route-preview] route_plan_failed user_id=%d code=%s message=%q origin=(%f,%f) destination=(%f,%f)",
				user.ID,
				strings.TrimSpace(routeServiceErr.Code),
				strings.TrimSpace(routeServiceErr.Message),
				request.Origin.Lat,
				request.Origin.Lon,
				request.Destination.Lat,
				request.Destination.Lon,
			)
			// Fallback path: still show a preview image using direct line when routing is unavailable.
			if routeServiceErr.Code == RouteErrorUpstreamFail || routeServiceErr.Code == RouteErrorUpstreamRetry || routeServiceErr.Code == RouteErrorUpstreamRate {
				routeResult = BuildFallbackRoutePlan(request.Origin, request.Destination, routeServiceErr.Message)
			}
		}
	}
	if routeResult == nil && routeErr != nil {
		if routeServiceErr, ok := routeErr.(*RouteServiceError); ok {
			statusCode := routeServiceErr.HTTPStatus
			if statusCode < http.StatusBadRequest {
				statusCode = http.StatusBadGateway
			}
			writeIntegrationError(w, statusCode, strings.TrimSpace(routeServiceErr.Code), strings.TrimSpace(routeServiceErr.Message))
			return
		}
		writeIntegrationError(w, http.StatusBadGateway, "route_preview_failed", "failed to plan route")
		return
	}

	imageResult, imageErr := GenerateTomTomRouteStaticImage(r.Context(), RoutePreviewImageInput{
		Origin:      request.Origin,
		Destination: request.Destination,
		Geometry:    strings.TrimSpace(routeResult.Geometry),
		DistanceM:   &routeResult.DistanceMeters,
	})
	if imageErr != nil {
		log.Printf("[schedule-route-preview] static_image_failed user_id=%d message=%q origin=(%f,%f) destination=(%f,%f)",
			user.ID,
			strings.TrimSpace(imageErr.Error()),
			request.Origin.Lat,
			request.Origin.Lon,
			request.Destination.Lat,
			request.Destination.Lon,
		)
		if routeServiceErr, ok := imageErr.(*RouteServiceError); ok {
			statusCode := routeServiceErr.HTTPStatus
			if statusCode < http.StatusBadRequest {
				statusCode = http.StatusBadGateway
			}
			writeIntegrationError(w, statusCode, strings.TrimSpace(routeServiceErr.Code), strings.TrimSpace(routeServiceErr.Message))
			return
		}
		writeIntegrationError(w, http.StatusBadGateway, "route_preview_failed", "failed to generate route image")
		return
	}

	respondJSON(w, http.StatusOK, scheduleRoutePreviewResponse{
		Route:       routeResult,
		ImageBase64: base64.StdEncoding.EncodeToString(imageResult.Image),
		MimeType:    imageResult.MimeType,
		Format:      imageResult.Format,
		Width:       imageResult.Width,
		Height:      imageResult.Height,
	})
}
