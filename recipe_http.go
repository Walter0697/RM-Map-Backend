package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/middleware"
	"mapmarker/backend/service"

	"github.com/go-chi/chi"
	"gorm.io/gorm"
)

type recipeUpsertRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	Ingredients  string `json:"ingredients"`
	Instructions string `json:"instructions"`
	ServingSize  string `json:"serving_size"`
	PrepMinutes  int    `json:"prep_minutes"`
	CookMinutes  int    `json:"cook_minutes"`
	Tags         string `json:"tags"`
}

type recipeResponse struct {
	ID           uint   `json:"id"`
	UserID       uint   `json:"user_id"`
	RelationID   uint   `json:"relation_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Ingredients  string `json:"ingredients"`
	Instructions string `json:"instructions"`
	ServingSize  string `json:"serving_size"`
	PrepMinutes  int    `json:"prep_minutes"`
	CookMinutes  int    `json:"cook_minutes"`
	Tags         string `json:"tags"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func listUserRecipesHandler(w http.ResponseWriter, r *http.Request) {
	user := middleware.ForContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	relation, err := service.GetCurrentRelation(*user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if relation == nil {
		http.Error(w, "selected relation is required", http.StatusBadRequest)
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	if limit > 200 {
		limit = 200
	}

	items := make([]dbmodel.Recipe, 0)
	if err := database.Connection.Where("relation_id = ?", relation.ID).Order("updated_at desc").Limit(limit).Find(&items).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]recipeResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, buildRecipeResponse(item))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": resp,
	})
}

func createUserRecipeHandler(w http.ResponseWriter, r *http.Request) {
	user := middleware.ForContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	relation, err := service.GetCurrentRelation(*user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if relation == nil {
		http.Error(w, "selected relation is required", http.StatusBadRequest)
		return
	}

	var req recipeUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	recipe, err := buildRecipeModel(nil, req, *user, *relation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := recipe.Create(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, buildRecipeResponse(*recipe))
}

func getUserRecipeHandler(w http.ResponseWriter, r *http.Request) {
	recipe, relation, statusCode, err := loadAuthorizedRecipe(r)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}
	if recipe.RelationID != relation.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, buildRecipeResponse(*recipe))
}

func updateUserRecipeHandler(w http.ResponseWriter, r *http.Request) {
	recipe, relation, statusCode, err := loadAuthorizedRecipe(r)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}
	if recipe.RelationID != relation.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req recipeUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user := middleware.ForContext(r.Context())
	updated, err := buildRecipeModel(recipe, req, *user, *relation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := updated.Update(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, buildRecipeResponse(*updated))
}

func deleteUserRecipeHandler(w http.ResponseWriter, r *http.Request) {
	recipe, relation, statusCode, err := loadAuthorizedRecipe(r)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}
	if recipe.RelationID != relation.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	user := middleware.ForContext(r.Context())
	recipe.UpdatedBy = user
	recipe.UpdatedUID = &user.ID
	if err := recipe.Delete(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
		"entity":  "recipe",
		"id":      recipe.ID,
	})
}

func buildRecipeResponse(recipe dbmodel.Recipe) recipeResponse {
	return recipeResponse{
		ID:           recipe.ID,
		UserID:       recipe.UserID,
		RelationID:   recipe.RelationID,
		Title:        recipe.Title,
		Description:  recipe.Description,
		Ingredients:  recipe.Ingredients,
		Instructions: recipe.Instructions,
		ServingSize:  recipe.ServingSize,
		PrepMinutes:  recipe.PrepMinutes,
		CookMinutes:  recipe.CookMinutes,
		Tags:         recipe.Tags,
		CreatedAt:    recipe.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    recipe.UpdatedAt.Format(time.RFC3339),
	}
}

func buildRecipeModel(existing *dbmodel.Recipe, req recipeUpsertRequest, user dbmodel.User, relation dbmodel.UserRelation) (*dbmodel.Recipe, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}
	if req.PrepMinutes < 0 || req.CookMinutes < 0 {
		return nil, errors.New("minutes cannot be negative")
	}

	recipe := existing
	if recipe == nil {
		recipe = &dbmodel.Recipe{}
		recipe.User = user
		recipe.UserID = user.ID
		recipe.CreatedBy = &user
		recipe.CreatedUID = &user.ID
		recipe.Relation = relation
		recipe.RelationID = relation.ID
	}

	recipe.Title = title
	recipe.Description = strings.TrimSpace(req.Description)
	recipe.Ingredients = strings.TrimSpace(req.Ingredients)
	recipe.Instructions = strings.TrimSpace(req.Instructions)
	recipe.ServingSize = strings.TrimSpace(req.ServingSize)
	recipe.PrepMinutes = req.PrepMinutes
	recipe.CookMinutes = req.CookMinutes
	recipe.Tags = strings.TrimSpace(req.Tags)
	recipe.UpdatedBy = &user
	recipe.UpdatedUID = &user.ID

	return recipe, nil
}

func loadAuthorizedRecipe(r *http.Request) (*dbmodel.Recipe, *dbmodel.UserRelation, int, error) {
	user := middleware.ForContext(r.Context())
	if user == nil {
		return nil, nil, http.StatusUnauthorized, errors.New("unauthorized")
	}

	relation, err := service.GetCurrentRelation(*user)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, err
	}
	if relation == nil {
		return nil, nil, http.StatusBadRequest, errors.New("selected relation is required")
	}

	idParam := strings.TrimSpace(chi.URLParam(r, "id"))
	recipeID, err := strconv.Atoi(idParam)
	if err != nil || recipeID <= 0 {
		return nil, nil, http.StatusBadRequest, errors.New("invalid recipe id")
	}

	recipe := &dbmodel.Recipe{}
	recipe.ID = uint(recipeID)
	if err := recipe.GetByID(database.Connection); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, http.StatusNotFound, errors.New("recipe not found")
		}
		return nil, nil, http.StatusInternalServerError, err
	}

	return recipe, relation, http.StatusOK, nil
}
