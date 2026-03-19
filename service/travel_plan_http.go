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

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

type integrationTravelPlanDailyInput struct {
	DayIndex  *int    `json:"day_index"`
	LocalDate *string `json:"local_date"`
	Summary   string  `json:"summary"`
	Details   *string `json:"details"`
}

type integrationCreateTravelPlanRequest struct {
	RelationID     uint                              `json:"relation_id"`
	UserID         uint                              `json:"user_id"`
	PlanRelationID string                            `json:"plan_relation_id"`
	Title          string                            `json:"title"`
	Description    string                            `json:"description"`
	StartDate      *string                           `json:"start_date"`
	EndDate        *string                           `json:"end_date"`
	DailyPlans     []integrationTravelPlanDailyInput `json:"daily_plans"`
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
	ID        uint    `json:"id"`
	DayIndex  int     `json:"day_index"`
	LocalDate *string `json:"local_date"`
	Summary   string  `json:"summary"`
	Details   string  `json:"details"`
}

type TravelPlanSummaryResponse struct {
	ID             uint    `json:"id"`
	UserID         uint    `json:"user_id"`
	Title          string  `json:"title"`
	PlanRelationID string  `json:"plan_relation_id"`
	StartDate      *string `json:"start_date"`
	EndDate        *string `json:"end_date"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type TravelPlanDetailResponse struct {
	TravelPlanSummaryResponse
	Description string                    `json:"description"`
	DailyPlans  []TravelPlanDailyResponse `json:"daily_plans"`
}

const defaultTravelPlanStatus = "draft"

func IntegrationListTravelPlansHandler(w http.ResponseWriter, r *http.Request) {
	allowedSort := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"start_date": "start_date",
		"end_date":   "end_date",
		"title":      "title",
	}
	queryOption, err := parseListQueryFromRequest(r, allowedSort, "created_at", []string{"title", "plan_relation_id", "status", "from", "to"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.travel-plans.list", constant.APIKeyScopeTravelPlansRead, queryContextString(queryOption))
	if !ok {
		return
	}

	query := database.Connection.Model(&dbmodel.TravelPlan{}).
		Select("travel_plans.*").
		Where("relation_id = ?", apiKey.Relation.ID)

	if value, ok := queryOption.Filters["plan_relation_id"]; ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			query = query.Where("plan_relation_id = ?", trimmed)
		}
	}
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

	apiKey, ok := authenticateIntegrationRequestJSON(w, r, "integration.travel-plans.create", constant.APIKeyScopeTravelPlansWrite, "")
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
	apiKey, ok := authenticateIntegrationRequest(w, r, "integration.travel-plans.get", constant.APIKeyScopeTravelPlansRead, "")
	if !ok {
		return
	}

	planID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || planID <= 0 {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}

	var plan dbmodel.TravelPlan
	plan.ID = uint(planID)
	if err := plan.GetWithDailyPlans(database.Connection); err != nil {
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

	respondJSON(w, http.StatusOK, BuildTravelPlanDetailResponse(plan))
}

func IntegrationUpdateTravelPlanHandler(w http.ResponseWriter, r *http.Request) {
	planID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || planID <= 0 {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}

	apiKey, ok := authenticateIntegrationRequestJSON(w, r, "integration.travel-plans.update", constant.APIKeyScopeTravelPlansWrite, "")
	if !ok {
		return
	}

	var req integrationUpdateTravelPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var plan dbmodel.TravelPlan
	plan.ID = uint(planID)
	if err := plan.GetWithDailyPlans(database.Connection); err != nil {
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

	if err := applyTravelPlanUpdates(&plan, req, apiKey); err != nil {
		writeTravelPlanError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, BuildTravelPlanDetailResponse(plan))
}

func createTravelPlan(apiKey *dbmodel.APIKey, req integrationCreateTravelPlanRequest) (*dbmodel.TravelPlan, error) {
	trimmedTitle := strings.TrimSpace(req.Title)
	if trimmedTitle == "" {
		return nil, validationError{message: "title is required"}
	}
	if req.UserID == 0 {
		return nil, validationError{message: "user_id is required"}
	}

	relationID := apiKey.Relation.ID
	if req.RelationID != 0 && req.RelationID != relationID {
		return nil, validationError{message: "relation_id mismatch"}
	}
	if err := ensureUserInRelation(apiKey.Relation, req.UserID); err != nil {
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
		RelationID:     relationID,
		UserID:         req.UserID,
		PlanRelationID: strings.TrimSpace(req.PlanRelationID),
		Title:          trimmedTitle,
		Description:    strings.TrimSpace(req.Description),
		StartDate:      startDate,
		EndDate:        endDate,
		Status:         defaultTravelPlanStatus,
	}
	if operator := actorUserFromAPIKey(apiKey); operator != nil {
		plan.CreatedBy = operator
		plan.UpdatedBy = operator
	}

	if err := database.Connection.Transaction(func(tx *gorm.DB) error {
		if err := plan.Create(tx); err != nil {
			return err
		}
		if err := persistTravelPlanDailyEntries(tx, plan.ID, req.DailyPlans); err != nil {
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
			if err := persistTravelPlanDailyEntries(tx, plan.ID, *req.DailyPlans); err != nil {
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

func persistTravelPlanDailyEntries(tx *gorm.DB, travelPlanID uint, inputs []integrationTravelPlanDailyInput) error {
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
		ID:             plan.ID,
		UserID:         plan.UserID,
		Title:          plan.Title,
		PlanRelationID: plan.PlanRelationID,
		StartDate:      formatDateForResponse(plan.StartDate),
		EndDate:        formatDateForResponse(plan.EndDate),
		Status:         plan.Status,
		CreatedAt:      plan.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      plan.UpdatedAt.Format(time.RFC3339),
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
		ID:        row.ID,
		DayIndex:  row.DayIndex,
		LocalDate: formatDateForResponse(row.LocalDate),
		Summary:   row.Summary,
		Details:   row.Details,
	}
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
