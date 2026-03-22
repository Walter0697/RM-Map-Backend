package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/utils"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

type integrationTravelPlanDailyInput struct {
	ScheduleID *uint   `json:"schedule_id"`
	DayIndex   *int    `json:"day_index"`
	LocalDate  *string `json:"local_date"`
	Summary    string  `json:"summary"`
	Details    *string `json:"details"`
}

type integrationCreateTravelPlanRequest struct {
	RelationID  uint                              `json:"relation_id"`
	UserID      uint                              `json:"user_id"`
	Username    string                            `json:"username"`
	Title       string                            `json:"title"`
	Description string                            `json:"description"`
	StartDate   *string                           `json:"start_date"`
	EndDate     *string                           `json:"end_date"`
	DailyPlans  []integrationTravelPlanDailyInput `json:"daily_plans"`
}

type integrationUpdateTravelPlanRequest struct {
	Title       *string                            `json:"title"`
	Description *string                            `json:"description"`
	StartDate   *string                            `json:"start_date"`
	EndDate     *string                            `json:"end_date"`
	Status      *string                            `json:"status"`
	DailyPlans  *[]integrationTravelPlanDailyInput `json:"daily_plans"`
}

type TravelPlanDailyResponse struct {
	ID         uint    `json:"id"`
	ScheduleID *uint   `json:"schedule_id"`
	DayIndex   int     `json:"day_index"`
	LocalDate  *string `json:"local_date"`
	Summary    string  `json:"summary"`
	Details    string  `json:"details"`
}

type TravelPlanSummaryResponse struct {
	ID        uint    `json:"id"`
	UserID    uint    `json:"user_id"`
	Title     string  `json:"title"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type TravelPlanDetailResponse struct {
	TravelPlanSummaryResponse
	Description string                    `json:"description"`
	DailyPlans  []TravelPlanDailyResponse `json:"daily_plans"`
}

const defaultTravelPlanStatus = "draft"

var integrationAuthenticateTravelPlanRequestFn = authenticateIntegrationRequest
var integrationAuthenticateTravelPlanRequestJSONFn = authenticateIntegrationRequestJSON

var integrationGetTravelPlanByIDFn = func(planID uint) (*dbmodel.TravelPlan, error) {
	plan := &dbmodel.TravelPlan{}
	plan.ID = planID
	if err := plan.GetWithDailyPlans(database.Connection); err != nil {
		return nil, err
	}
	return plan, nil
}

var integrationDeleteTravelPlanFn = func(plan *dbmodel.TravelPlan, actor *dbmodel.User) error {
	return database.Connection.Transaction(func(tx *gorm.DB) error {
		if actor != nil {
			plan.UpdatedBy = actor
			if err := plan.Update(tx); err != nil {
				return err
			}
		}
		if err := tx.Where("travel_plan_id = ?", plan.ID).Delete(&dbmodel.TravelPlanDailyPlan{}).Error; err != nil {
			return err
		}
		return tx.Delete(&dbmodel.TravelPlan{}, plan.ID).Error
	})
}

var integrationGetTravelPlanDailyByIDFn = func(planID uint, dailyID uint) (*dbmodel.TravelPlanDailyPlan, error) {
	daily := &dbmodel.TravelPlanDailyPlan{}
	if err := database.Connection.Where("id = ? AND travel_plan_id = ?", dailyID, planID).First(daily).Error; err != nil {
		return nil, err
	}
	return daily, nil
}

var integrationDeleteTravelPlanDailyByIDFn = func(planID uint, dailyID uint) error {
	return database.Connection.Where("id = ? AND travel_plan_id = ?", dailyID, planID).Delete(&dbmodel.TravelPlanDailyPlan{}).Error
}

func IntegrationListTravelPlansHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"start_date": "start_date",
		"end_date":   "end_date",
		"title":      "title",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "created_at", []string{"title", "status", "from", "to", "username"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey, ok := integrationAuthenticateTravelPlanRequestFn(w, r, "integration.travel-plans.list", constant.APIKeyScopeTravelPlansRead, queryContextString(queryOption))
	if !ok {
		return
	}

	query := database.Connection.Model(&dbmodel.TravelPlan{}).
		Where("relation_id = ?", apiKey.Relation.ID)

	if value, ok := queryOption.Filters["title"]; ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			query = query.Where("title ILIKE ?", "%"+trimmed+"%")
		}
	}
	if value, ok := queryOption.Filters["status"]; ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			query = query.Where("status = ?", trimmed)
		}
	}
	if value, ok := queryOption.Filters["username"]; ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			resolvedUserID, resolveErr := resolveTravelPlanUsernameInRelation(apiKey.Relation, trimmed)
			if resolveErr != nil {
				http.Error(w, resolveErr.Error(), http.StatusBadRequest)
				return
			}
			query = query.Where("user_id = ?", resolvedUserID)
		}
	}
	if value, ok := queryOption.Filters["from"]; ok {
		if parsed, parseErr := parseDateString(value); parseErr == nil {
			query = query.Where("start_date >= ?", parsed.Format("2006-01-02"))
		} else {
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}
	}
	if value, ok := queryOption.Filters["to"]; ok {
		if parsed, parseErr := parseDateString(value); parseErr == nil {
			query = query.Where("end_date <= ?", parsed.Format("2006-01-02"))
		} else {
			http.Error(w, "invalid to", http.StatusBadRequest)
			return
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plans := make([]dbmodel.TravelPlan, 0)
	queryWithSort := query
	if queryOption.Cursor > 0 {
		queryWithSort = queryWithSort.Where("id > ?", queryOption.Cursor)
		queryWithSort = queryWithSort.Order("id asc")
	} else {
		queryWithSort = queryWithSort.Order(sortClause(queryOption, allowedSort)).Order("id asc")
	}
	if err := queryWithSort.Limit(queryOption.Limit).Offset(queryOption.Offset).Find(&plans).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]TravelPlanSummaryResponse, 0, len(plans))
	for _, plan := range plans {
		response = append(response, BuildTravelPlanSummaryResponse(plan))
	}
	nextCursor := ""
	if len(plans) == queryOption.Limit {
		nextCursor = strconv.FormatUint(uint64(plans[len(plans)-1].ID), 10)
	}

	respondJSON(w, http.StatusOK, integrationListResponse(response, total, queryOption, nextCursor))
}
func IntegrationCreateTravelPlanHandler(w http.ResponseWriter, r *http.Request) {
	var req integrationCreateTravelPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	apiKey, ok := integrationAuthenticateTravelPlanRequestJSONFn(w, r, "integration.travel-plans.create", constant.APIKeyScopeTravelPlansWrite, "")
	if !ok {
		return
	}

	plan, err := createTravelPlan(apiKey, req)
	if err != nil {
		writeTravelPlanError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, BuildTravelPlanDetailResponse(*plan))
}

func IntegrationGetTravelPlanHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := integrationAuthenticateTravelPlanRequestFn(w, r, "integration.travel-plans.get", constant.APIKeyScopeTravelPlansRead, "")
	if !ok {
		return
	}

	planID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || planID <= 0 {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}

	plan, err := integrationGetTravelPlanByIDFn(uint(planID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "plan not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if plan.RelationID != apiKey.Relation.ID {
		http.Error(w, "plan does not belong to the api key relation", http.StatusForbidden)
		return
	}

	respondJSON(w, http.StatusOK, BuildTravelPlanDetailResponse(*plan))
}

func IntegrationUpdateTravelPlanHandler(w http.ResponseWriter, r *http.Request) {
	planID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || planID <= 0 {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}

	apiKey, ok := integrationAuthenticateTravelPlanRequestJSONFn(w, r, "integration.travel-plans.update", constant.APIKeyScopeTravelPlansWrite, "")
	if !ok {
		return
	}

	var req integrationUpdateTravelPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	plan, err := integrationGetTravelPlanByIDFn(uint(planID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "plan not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if plan.RelationID != apiKey.Relation.ID {
		http.Error(w, "plan does not belong to the api key relation", http.StatusForbidden)
		return
	}

	if err := applyTravelPlanUpdates(plan, req, apiKey); err != nil {
		writeTravelPlanError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, BuildTravelPlanDetailResponse(*plan))
}

func IntegrationDeleteTravelPlanHandler(w http.ResponseWriter, r *http.Request) {
	planID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || planID <= 0 {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}

	apiKey, ok := integrationAuthenticateTravelPlanRequestJSONFn(w, r, "integration.travel-plans.delete", constant.APIKeyScopeTravelPlansWrite, "")
	if !ok {
		return
	}

	plan, err := integrationGetTravelPlanByIDFn(uint(planID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "plan not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if plan.RelationID != apiKey.Relation.ID {
		http.Error(w, "plan does not belong to the api key relation", http.StatusForbidden)
		return
	}

	if err := integrationDeleteTravelPlanFn(plan, actorUserFromAPIKey(apiKey)); err != nil {
		writeTravelPlanError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
		"entity":  "travel_plan",
		"id":      plan.ID,
	})
}

func IntegrationDeleteTravelPlanDailyPlanHandler(w http.ResponseWriter, r *http.Request) {
	planID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || planID <= 0 {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}
	dailyID, err := strconv.Atoi(chi.URLParam(r, "daily_id"))
	if err != nil || dailyID <= 0 {
		http.Error(w, "invalid daily plan id", http.StatusBadRequest)
		return
	}

	apiKey, ok := integrationAuthenticateTravelPlanRequestJSONFn(w, r, "integration.travel-plans.daily.delete", constant.APIKeyScopeTravelPlansWrite, "")
	if !ok {
		return
	}

	plan, err := integrationGetTravelPlanByIDFn(uint(planID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "plan not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if plan.RelationID != apiKey.Relation.ID {
		http.Error(w, "plan does not belong to the api key relation", http.StatusForbidden)
		return
	}

	if _, err := integrationGetTravelPlanDailyByIDFn(plan.ID, uint(dailyID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "daily plan not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := integrationDeleteTravelPlanDailyByIDFn(plan.ID, uint(dailyID)); err != nil {
		writeTravelPlanError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deleted":        true,
		"entity":         "travel_plan_daily",
		"id":             dailyID,
		"travel_plan_id": plan.ID,
	})
}

func createTravelPlan(apiKey *dbmodel.APIKey, req integrationCreateTravelPlanRequest) (*dbmodel.TravelPlan, error) {
	trimmedTitle := strings.TrimSpace(req.Title)
	if trimmedTitle == "" {
		return nil, validationError{message: "title is required"}
	}

	relationID := apiKey.Relation.ID
	if req.RelationID != 0 && req.RelationID != relationID {
		return nil, validationError{message: "relation_id mismatch"}
	}
	resolvedUserID, err := resolveTravelPlanCreateUserID(apiKey.Relation, req.UserID, req.Username)
	if err != nil {
		return nil, err
	}

	startDate, err := parseOptionalDate(req.StartDate)
	if err != nil {
		return nil, err
	}
	endDate, err := parseOptionalDate(req.EndDate)
	if err != nil {
		return nil, err
	}

	plan := &dbmodel.TravelPlan{
		RelationID:  relationID,
		UserID:      resolvedUserID,
		Title:       trimmedTitle,
		Description: strings.TrimSpace(req.Description),
		StartDate:   startDate,
		EndDate:     endDate,
		Status:      defaultTravelPlanStatus,
	}
	if operator := actorUserFromAPIKey(apiKey); operator != nil {
		plan.CreatedBy = operator
		plan.UpdatedBy = operator
	}

	if err := database.Connection.Transaction(func(tx *gorm.DB) error {
		if err := plan.Create(tx); err != nil {
			return err
		}
		if err := persistTravelPlanDailyEntries(tx, plan.ID, relationID, req.DailyPlans); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := plan.GetWithDailyPlans(database.Connection); err != nil {
		return nil, err
	}

	return plan, nil
}

func applyTravelPlanUpdates(plan *dbmodel.TravelPlan, req integrationUpdateTravelPlanRequest, apiKey *dbmodel.APIKey) error {
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			return validationError{message: "title cannot be empty"}
		}
		plan.Title = trimmed
	}
	if req.Description != nil {
		plan.Description = strings.TrimSpace(*req.Description)
	}
	if req.StartDate != nil {
		parsed, err := parseOptionalDate(req.StartDate)
		if err != nil {
			return err
		}
		plan.StartDate = parsed
	}
	if req.EndDate != nil {
		parsed, err := parseOptionalDate(req.EndDate)
		if err != nil {
			return err
		}
		plan.EndDate = parsed
	}
	if req.Status != nil {
		plan.Status = strings.TrimSpace(*req.Status)
	}
	if operator := actorUserFromAPIKey(apiKey); operator != nil {
		plan.UpdatedBy = operator
	}

	if err := database.Connection.Transaction(func(tx *gorm.DB) error {
		if err := plan.Update(tx); err != nil {
			return err
		}
		if req.DailyPlans != nil {
			if err := persistTravelPlanDailyEntries(tx, plan.ID, plan.RelationID, *req.DailyPlans); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if err := plan.GetWithDailyPlans(database.Connection); err != nil {
		return err
	}

	return nil
}

func persistTravelPlanDailyEntries(tx *gorm.DB, travelPlanID uint, relationID uint, inputs []integrationTravelPlanDailyInput) error {
	if err := tx.Where("travel_plan_id = ?", travelPlanID).Delete(&dbmodel.TravelPlanDailyPlan{}).Error; err != nil {
		return err
	}

	for index, input := range inputs {
		summary := strings.TrimSpace(input.Summary)
		if summary == "" {
			return validationError{message: fmt.Sprintf("daily plan summary is required for row %d", index+1)}
		}

		dayIndex := index + 1
		if input.DayIndex != nil && *input.DayIndex > 0 {
			dayIndex = *input.DayIndex
		}

		daily := dbmodel.TravelPlanDailyPlan{
			TravelPlanID: travelPlanID,
			DayIndex:     dayIndex,
			Summary:      summary,
		}
		if scheduleID, err := validateOptionalScheduleID(tx, relationID, input.ScheduleID); err != nil {
			return err
		} else {
			daily.ScheduleID = scheduleID
		}
		if input.Details != nil {
			daily.Details = strings.TrimSpace(*input.Details)
		}
		if parsed, err := parseOptionalDate(input.LocalDate); err != nil {
			return err
		} else {
			daily.LocalDate = parsed
		}

		if err := tx.Create(&daily).Error; err != nil {
			return err
		}
	}

	return nil
}

func BuildTravelPlanSummaryResponse(plan dbmodel.TravelPlan) TravelPlanSummaryResponse {
	return TravelPlanSummaryResponse{
		ID:        plan.ID,
		UserID:    plan.UserID,
		Title:     plan.Title,
		StartDate: formatDateForResponse(plan.StartDate),
		EndDate:   formatDateForResponse(plan.EndDate),
		Status:    plan.Status,
		CreatedAt: plan.CreatedAt.Format(time.RFC3339),
		UpdatedAt: plan.UpdatedAt.Format(time.RFC3339),
	}
}

func BuildTravelPlanDetailResponse(plan dbmodel.TravelPlan) TravelPlanDetailResponse {
	detail := TravelPlanDetailResponse{
		TravelPlanSummaryResponse: BuildTravelPlanSummaryResponse(plan),
		Description:               plan.Description,
		DailyPlans:                make([]TravelPlanDailyResponse, 0, len(plan.DailyPlans)),
	}
	for _, daily := range plan.DailyPlans {
		detail.DailyPlans = append(detail.DailyPlans, buildTravelPlanDailyResponse(daily))
	}
	return detail
}

func buildTravelPlanDailyResponse(row dbmodel.TravelPlanDailyPlan) TravelPlanDailyResponse {
	return TravelPlanDailyResponse{
		ID:         row.ID,
		ScheduleID: row.ScheduleID,
		DayIndex:   row.DayIndex,
		LocalDate:  formatDateForResponse(row.LocalDate),
		Summary:    row.Summary,
		Details:    row.Details,
	}
}

func validateOptionalScheduleID(tx *gorm.DB, relationID uint, scheduleID *uint) (*uint, error) {
	if scheduleID == nil {
		return nil, nil
	}
	if *scheduleID == 0 {
		return nil, nil
	}

	var schedule dbmodel.Schedule
	if err := tx.Where("id = ? AND relation_id = ?", *scheduleID, relationID).First(&schedule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, validationError{message: fmt.Sprintf("schedule_id %d does not exist for the relation", *scheduleID)}
		}
		return nil, err
	}

	resolved := schedule.ID
	return &resolved, nil
}

func formatDateForResponse(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}

func parseOptionalDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := parseDateString(trimmed)
	if err != nil {
		return nil, validationError{message: err.Error()}
	}
	return &parsed, nil
}

func parseDateString(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid date format")
}

type validationError struct {
	message string
}

func (v validationError) Error() string {
	return v.message
}

func ensureUserInRelation(relation dbmodel.UserRelation, userID uint) error {
	if userID == 0 {
		return validationError{message: "user_id is required"}
	}
	if relation.ID == 0 {
		return validationError{message: "relation context missing"}
	}
	if userID != relation.UserOneUID && userID != relation.UserTwoUID {
		return validationError{message: "user_id does not belong to the relation"}
	}
	return nil
}

func resolveTravelPlanCreateUserID(relation dbmodel.UserRelation, userID uint, username string) (uint, error) {
	if userID != 0 {
		if err := ensureUserInRelation(relation, userID); err != nil {
			return 0, err
		}
		if strings.TrimSpace(username) == "" {
			return userID, nil
		}
		var user dbmodel.User
		user.Username = strings.TrimSpace(username)
		if err := user.GetUserByUsername(database.Connection); err != nil {
			if utils.RecordNotFound(err) {
				return 0, validationError{message: "username does not exist"}
			}
			return 0, err
		}
		if user.ID != userID {
			return 0, validationError{message: "username does not match user_id"}
		}
		return userID, nil
	}

	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return 0, validationError{message: "username is required"}
	}
	return resolveTravelPlanUsernameInRelation(relation, trimmedUsername)
}

func resolveTravelPlanUsernameInRelation(relation dbmodel.UserRelation, username string) (uint, error) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return 0, validationError{message: "username is required"}
	}

	var user dbmodel.User
	user.Username = trimmedUsername
	if err := user.GetUserByUsername(database.Connection); err != nil {
		if utils.RecordNotFound(err) {
			return 0, validationError{message: "username does not exist"}
		}
		return 0, err
	}
	if err := ensureUserInRelation(relation, user.ID); err != nil {
		return 0, err
	}
	return user.ID, nil
}

func actorUserFromAPIKey(apiKey *dbmodel.APIKey) *dbmodel.User {
	if apiKey == nil {
		return nil
	}
	if apiKey.ActorUserID != nil && apiKey.ActorUser.ID != 0 {
		return &apiKey.ActorUser
	}
	return nil
}

func writeTravelPlanError(w http.ResponseWriter, err error) {
	var validation validationError
	if errors.As(err, &validation) {
		http.Error(w, validation.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
