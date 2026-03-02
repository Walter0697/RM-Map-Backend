package service

import (
	"encoding/json"
	"fmt"
	"log"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
)

const (
	cleanupEntityMarker   = "marker"
	cleanupEntitySchedule = "schedule"
)

type adminCleanupDeleteRequest struct {
	ConfirmID    int    `json:"confirm_id"`
	ConfirmLabel string `json:"confirm_label"`
	Reason       string `json:"reason"`
}

type adminCleanupScheduleRequest struct {
	EntityType   string `json:"entity_type"`
	TargetID     int    `json:"target_id"`
	ConfirmID    int    `json:"confirm_id"`
	ConfirmLabel string `json:"confirm_label"`
	ExecuteAt    string `json:"execute_at"`
	Reason       string `json:"reason"`
}

type adminCleanupMarkerResponse struct {
	ID          uint       `json:"id"`
	Label       string     `json:"label"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	RelationID  uint       `json:"relation_id"`
	Country     string     `json:"country"`
	CountryCode string     `json:"country_code"`
	ToTime      *time.Time `json:"to_time"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type adminCleanupScheduleResponse struct {
	ID           uint      `json:"id"`
	Label        string    `json:"label"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	RelationID   uint      `json:"relation_id"`
	MarkerID     *uint     `json:"marker_id"`
	MarkerLabel  string    `json:"marker_label"`
	SelectedDate time.Time `json:"selected_date"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type adminCleanupJobResponse struct {
	ID           uint      `json:"id"`
	EntityType   string    `json:"entity_type"`
	TargetID     uint      `json:"target_id"`
	ConfirmLabel string    `json:"confirm_label"`
	Reason       string    `json:"reason"`
	ExecuteAt    time.Time `json:"execute_at"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

var cleanupCurrentUserFn = currentUserFromRequest

var cleanupListMarkersFn = func(queryOption integrationListQuery) ([]adminCleanupMarkerResponse, int64, error) {
	query := database.Connection.Model(&dbmodel.Marker{})

	if value, ok := queryOption.Filters["type"]; ok {
		query = query.Where("type = ?", value)
	}
	if value, ok := queryOption.Filters["status"]; ok {
		query = query.Where("status = ?", value)
	}
	if value, ok := queryOption.Filters["country"]; ok {
		query = query.Where("country = ?", value)
	}
	if value, ok := queryOption.Filters["country_code"]; ok {
		query = query.Where("country_code = ?", value)
	}
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("label ILIKE ?", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["search"]; ok {
		keyword := "%" + value + "%"
		query = query.Where("label ILIKE ? OR address ILIKE ? OR description ILIKE ?", keyword, keyword, keyword)
	}
	if value, ok := queryOption.Filters["relation_id"]; ok {
		relationID, err := strconv.Atoi(value)
		if err != nil || relationID <= 0 {
			return nil, 0, fmt.Errorf("invalid relation_id")
		}
		query = query.Where("relation_id = ?", relationID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]dbmodel.Marker, 0)
	if err := query.Order(sortClause(queryOption, map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"label":      "label",
		"type":       "type",
		"status":     "status",
		"to_time":    "to_time",
	})).Limit(queryOption.Limit).Offset(queryOption.Offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	response := make([]adminCleanupMarkerResponse, 0, len(items))
	for _, item := range items {
		response = append(response, adminCleanupMarkerResponse{
			ID:          item.ID,
			Label:       item.Label,
			Type:        item.Type,
			Status:      item.Status,
			RelationID:  item.RelationId,
			Country:     item.Country,
			CountryCode: item.CountryCode,
			ToTime:      item.ToTime,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return response, total, nil
}

var cleanupListSchedulesFn = func(queryOption integrationListQuery) ([]adminCleanupScheduleResponse, int64, error) {
	query := database.Connection.Model(&dbmodel.Schedule{}).Preload("SelectedMarker")

	if value, ok := queryOption.Filters["status"]; ok {
		query = query.Where("status = ?", value)
	}
	if value, ok := queryOption.Filters["marker_id"]; ok {
		markerID, err := strconv.Atoi(value)
		if err != nil || markerID <= 0 {
			return nil, 0, fmt.Errorf("invalid marker_id")
		}
		query = query.Where("marker_id = ?", markerID)
	}
	if value, ok := queryOption.Filters["label"]; ok {
		query = query.Where("label ILIKE ?", "%"+value+"%")
	}
	if value, ok := queryOption.Filters["search"]; ok {
		keyword := "%" + value + "%"
		query = query.Where("label ILIKE ? OR description ILIKE ?", keyword, keyword)
	}
	if value, ok := queryOption.Filters["relation_id"]; ok {
		relationID, err := strconv.Atoi(value)
		if err != nil || relationID <= 0 {
			return nil, 0, fmt.Errorf("invalid relation_id")
		}
		query = query.Where("relation_id = ?", relationID)
	}
	if value, ok := queryOption.Filters["from"]; ok {
		fromTime, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid from, expected RFC3339")
		}
		query = query.Where("selected_date >= ?", fromTime.Format(time.RFC3339))
	}
	if value, ok := queryOption.Filters["to"]; ok {
		toTime, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid to, expected RFC3339")
		}
		query = query.Where("selected_date <= ?", toTime.Format(time.RFC3339))
	}
	if value, ok := queryOption.Filters["time"]; ok {
		day, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid time format, expected YYYY-MM-DD")
		}
		nextDay := day.AddDate(0, 0, 1)
		query = query.Where("selected_date >= ? AND selected_date < ?", day.Format(time.RFC3339), nextDay.Format(time.RFC3339))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]dbmodel.Schedule, 0)
	if err := query.Order(sortClause(queryOption, map[string]string{
		"created_at":    "created_at",
		"updated_at":    "updated_at",
		"selected_date": "selected_date",
		"label":         "label",
		"status":        "status",
	})).Limit(queryOption.Limit).Offset(queryOption.Offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	response := make([]adminCleanupScheduleResponse, 0, len(items))
	for _, item := range items {
		markerLabel := ""
		if item.SelectedMarker != nil {
			markerLabel = item.SelectedMarker.Label
		}
		response = append(response, adminCleanupScheduleResponse{
			ID:           item.ID,
			Label:        item.Label,
			Description:  item.Description,
			Status:       item.Status,
			RelationID:   item.RelationId,
			MarkerID:     item.MarkerId,
			MarkerLabel:  markerLabel,
			SelectedDate: item.SelectedDate,
			UpdatedAt:    item.UpdatedAt,
		})
	}

	return response, total, nil
}

var cleanupFindMarkerByIDFn = func(id uint) (*dbmodel.Marker, error) {
	marker := dbmodel.Marker{}
	marker.ID = id
	if err := marker.GetById(database.Connection); err != nil {
		return nil, err
	}
	return &marker, nil
}

var cleanupFindScheduleByIDFn = func(id uint) (*dbmodel.Schedule, error) {
	schedule := dbmodel.Schedule{}
	schedule.ID = id
	if err := schedule.GetById(database.Connection); err != nil {
		return nil, err
	}
	return &schedule, nil
}

var cleanupDeleteMarkerByIDFn = func(id uint) error {
	return database.Connection.Delete(&dbmodel.Marker{}, id).Error
}

var cleanupDeleteScheduleByIDFn = func(id uint) error {
	return database.Connection.Delete(&dbmodel.Schedule{}, id).Error
}

var cleanupValidateMarkerDeletionFn = func(id uint) error {
	var count int64
	if err := database.Connection.Model(&dbmodel.Schedule{}).Where("marker_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("marker has %d schedule(s); delete schedules first", count)
	}
	return nil
}

var cleanupCreateJobFn = func(request adminCleanupScheduleRequest, executeAt time.Time, actor dbmodel.User) (*adminCleanupJobResponse, error) {
	job := dbmodel.AdminCleanupJob{
		EntityType:   request.EntityType,
		TargetID:     uint(request.TargetID),
		ConfirmLabel: strings.TrimSpace(request.ConfirmLabel),
		Reason:       strings.TrimSpace(request.Reason),
		ExecuteAt:    executeAt,
		Status:       "pending",
		Outcome:      "",
		ObjectBase: dbmodel.ObjectBase{
			CreatedBy: &actor,
			UpdatedBy: &actor,
		},
	}
	if err := job.Create(database.Connection); err != nil {
		return nil, err
	}

	return &adminCleanupJobResponse{
		ID:           job.ID,
		EntityType:   job.EntityType,
		TargetID:     job.TargetID,
		ConfirmLabel: job.ConfirmLabel,
		Reason:       job.Reason,
		ExecuteAt:    job.ExecuteAt,
		Status:       job.Status,
		CreatedAt:    job.CreatedAt,
	}, nil
}

func requireCleanupAdmin(w http.ResponseWriter, r *http.Request) *dbmodel.User {
	user := cleanupCurrentUserFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return nil
	}
	if strings.TrimSpace(strings.ToLower(user.Role)) != "admin" {
		http.Error(w, "permission denied", http.StatusForbidden)
		return nil
	}
	return user
}

func AdminCleanupListMarkersHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"label":      "label",
		"type":       "type",
		"status":     "status",
		"to_time":    "to_time",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "updated_at", []string{"type", "status", "country", "country_code", "label", "search", "relation_id"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if requireCleanupAdmin(w, r) == nil {
		return
	}

	items, total, err := cleanupListMarkersFn(queryOption)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption))
}

func AdminCleanupListSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"created_at":    "created_at",
		"updated_at":    "updated_at",
		"selected_date": "selected_date",
		"label":         "label",
		"status":        "status",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "selected_date", []string{"status", "marker_id", "label", "search", "from", "to", "time", "relation_id"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if requireCleanupAdmin(w, r) == nil {
		return
	}

	items, total, err := cleanupListSchedulesFn(queryOption)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, integrationListResponse(items, total, queryOption))
}

func AdminCleanupDeleteMarkerHandler(w http.ResponseWriter, r *http.Request) {
	actor := requireCleanupAdmin(w, r)
	if actor == nil {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid marker id", http.StatusBadRequest)
		return
	}

	request := adminCleanupDeleteRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.ConfirmID != id {
		http.Error(w, "confirmation id mismatch", http.StatusBadRequest)
		cleanupAuditLog("delete", cleanupEntityMarker, uint(id), actor.ID, false, "confirmation id mismatch")
		return
	}

	marker, err := cleanupFindMarkerByIDFn(uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		cleanupAuditLog("delete", cleanupEntityMarker, uint(id), actor.ID, false, err.Error())
		return
	}
	if strings.TrimSpace(request.ConfirmLabel) != strings.TrimSpace(marker.Label) {
		http.Error(w, "confirmation label mismatch", http.StatusBadRequest)
		cleanupAuditLog("delete", cleanupEntityMarker, marker.ID, actor.ID, false, "confirmation label mismatch")
		return
	}

	if err := cleanupValidateMarkerDeletionFn(marker.ID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		cleanupAuditLog("delete", cleanupEntityMarker, marker.ID, actor.ID, false, err.Error())
		return
	}

	if err := cleanupDeleteMarkerByIDFn(marker.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		cleanupAuditLog("delete", cleanupEntityMarker, marker.ID, actor.ID, false, err.Error())
		return
	}

	cleanupAuditLog("delete", cleanupEntityMarker, marker.ID, actor.ID, true, strings.TrimSpace(request.Reason))
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "deleted",
		"entity_type": cleanupEntityMarker,
		"id":          marker.ID,
	})
}

func AdminCleanupDeleteScheduleHandler(w http.ResponseWriter, r *http.Request) {
	actor := requireCleanupAdmin(w, r)
	if actor == nil {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid schedule id", http.StatusBadRequest)
		return
	}

	request := adminCleanupDeleteRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.ConfirmID != id {
		http.Error(w, "confirmation id mismatch", http.StatusBadRequest)
		cleanupAuditLog("delete", cleanupEntitySchedule, uint(id), actor.ID, false, "confirmation id mismatch")
		return
	}

	schedule, err := cleanupFindScheduleByIDFn(uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		cleanupAuditLog("delete", cleanupEntitySchedule, uint(id), actor.ID, false, err.Error())
		return
	}
	if strings.TrimSpace(request.ConfirmLabel) != strings.TrimSpace(schedule.Label) {
		http.Error(w, "confirmation label mismatch", http.StatusBadRequest)
		cleanupAuditLog("delete", cleanupEntitySchedule, schedule.ID, actor.ID, false, "confirmation label mismatch")
		return
	}

	if err := cleanupDeleteScheduleByIDFn(schedule.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		cleanupAuditLog("delete", cleanupEntitySchedule, schedule.ID, actor.ID, false, err.Error())
		return
	}

	cleanupAuditLog("delete", cleanupEntitySchedule, schedule.ID, actor.ID, true, strings.TrimSpace(request.Reason))
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "deleted",
		"entity_type": cleanupEntitySchedule,
		"id":          schedule.ID,
	})
}

func AdminCleanupScheduleJobHandler(w http.ResponseWriter, r *http.Request) {
	actor := requireCleanupAdmin(w, r)
	if actor == nil {
		return
	}

	request := adminCleanupScheduleRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request.EntityType = strings.ToLower(strings.TrimSpace(request.EntityType))
	if request.EntityType != cleanupEntityMarker && request.EntityType != cleanupEntitySchedule {
		http.Error(w, "entity_type must be marker or schedule", http.StatusBadRequest)
		return
	}
	if request.TargetID <= 0 {
		http.Error(w, "target_id must be positive", http.StatusBadRequest)
		return
	}
	if request.ConfirmID != request.TargetID {
		http.Error(w, "confirmation id mismatch", http.StatusBadRequest)
		cleanupAuditLog("schedule", request.EntityType, uint(request.TargetID), actor.ID, false, "confirmation id mismatch")
		return
	}
	if strings.TrimSpace(request.ConfirmLabel) == "" {
		http.Error(w, "confirm_label is required", http.StatusBadRequest)
		return
	}
	executeAt, err := time.Parse(time.RFC3339, strings.TrimSpace(request.ExecuteAt))
	if err != nil {
		http.Error(w, "execute_at must be RFC3339", http.StatusBadRequest)
		return
	}
	if executeAt.Before(time.Now().Add(30 * time.Second)) {
		http.Error(w, "execute_at must be in the future", http.StatusBadRequest)
		return
	}

	targetLabel, err := cleanupTargetLabel(request.EntityType, uint(request.TargetID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		cleanupAuditLog("schedule", request.EntityType, uint(request.TargetID), actor.ID, false, err.Error())
		return
	}
	if strings.TrimSpace(request.ConfirmLabel) != strings.TrimSpace(targetLabel) {
		http.Error(w, "confirmation label mismatch", http.StatusBadRequest)
		cleanupAuditLog("schedule", request.EntityType, uint(request.TargetID), actor.ID, false, "confirmation label mismatch")
		return
	}

	response, err := cleanupCreateJobFn(request, executeAt, *actor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		cleanupAuditLog("schedule", request.EntityType, uint(request.TargetID), actor.ID, false, err.Error())
		return
	}

	cleanupAuditLog("schedule", request.EntityType, uint(request.TargetID), actor.ID, true, strings.TrimSpace(request.Reason))
	respondJSON(w, http.StatusCreated, response)
}

func cleanupTargetLabel(entityType string, id uint) (string, error) {
	if entityType == cleanupEntityMarker {
		item, err := cleanupFindMarkerByIDFn(id)
		if err != nil {
			return "", err
		}
		return item.Label, nil
	}
	if entityType == cleanupEntitySchedule {
		item, err := cleanupFindScheduleByIDFn(id)
		if err != nil {
			return "", err
		}
		return item.Label, nil
	}

	return "", fmt.Errorf("unsupported entity type")
}

func cleanupAuditLog(actionType string, entityType string, targetID uint, actorID uint, success bool, outcome string) {
	log.Printf(
		"cleanup_audit action=%s entity=%s target_id=%d actor_id=%d ts=%s success=%t outcome=%q",
		strings.TrimSpace(actionType),
		strings.TrimSpace(entityType),
		targetID,
		actorID,
		time.Now().UTC().Format(time.RFC3339),
		success,
		strings.TrimSpace(outcome),
	)
}
