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

type recipeIngredientInput struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
	Notes    string `json:"notes"`
}

type recipeStepInput struct {
	Instruction string `json:"instruction"`
}

type createRecipeRequest struct {
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Ingredients []recipeIngredientInput `json:"ingredients"`
	Steps       []recipeStepInput       `json:"steps"`
}

type updateRecipeRequest struct {
	Title       *string                  `json:"title"`
	Description *string                  `json:"description"`
	Ingredients *[]recipeIngredientInput `json:"ingredients"`
	Steps       *[]recipeStepInput       `json:"steps"`
}

type RecipeIngredientResponse struct {
	ID        uint   `json:"id"`
	SortOrder int    `json:"sort_order"`
	Name      string `json:"name"`
	Quantity  string `json:"quantity"`
	Notes     string `json:"notes"`
}

type RecipeStepResponse struct {
	ID          uint   `json:"id"`
	SortOrder   int    `json:"sort_order"`
	Instruction string `json:"instruction"`
}

type RecipeSummaryResponse struct {
	ID              uint   `json:"id"`
	UserID          uint   `json:"user_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	IngredientCount int    `json:"ingredient_count"`
	StepCount       int    `json:"step_count"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type RecipeDetailResponse struct {
	RecipeSummaryResponse
	Ingredients []RecipeIngredientResponse `json:"ingredients"`
	Steps       []RecipeStepResponse       `json:"steps"`
}

type recipeValidationError struct {
	message string
}

func (v recipeValidationError) Error() string {
	return v.message
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

	recipes := make([]dbmodel.Recipe, 0)
	if err := database.Connection.
		Preload("Ingredients").
		Preload("Steps").
		Where("relation_id = ?", relation.ID).
		Order("updated_at desc").
		Find(&recipes).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]RecipeSummaryResponse, 0, len(recipes))
	for _, recipe := range recipes {
		items = append(items, buildRecipeSummaryResponse(recipe))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
	})
}

func getUserRecipeHandler(w http.ResponseWriter, r *http.Request) {
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

	recipe, err := getRecipeFromRequest(r)
	if err != nil {
		writeRecipeError(w, err)
		return
	}
	if recipe.RelationID != relation.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, buildRecipeDetailResponse(*recipe))
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

	var req createRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	recipe, err := createRecipe(*user, *relation, req)
	if err != nil {
		writeRecipeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, buildRecipeDetailResponse(*recipe))
}

func updateUserRecipeHandler(w http.ResponseWriter, r *http.Request) {
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

	var req updateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	recipe, err := getRecipeFromRequest(r)
	if err != nil {
		writeRecipeError(w, err)
		return
	}
	if recipe.RelationID != relation.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if recipe.UserID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := applyRecipeUpdates(recipe, req, *user); err != nil {
		writeRecipeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildRecipeDetailResponse(*recipe))
}

func deleteUserRecipeHandler(w http.ResponseWriter, r *http.Request) {
	user := middleware.ForContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	recipe, err := getRecipeFromRequest(r)
	if err != nil {
		writeRecipeError(w, err)
		return
	}
	if recipe.UserID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := database.Connection.Transaction(func(tx *gorm.DB) error {
		recipe.UpdatedBy = user
		if err := recipe.Update(tx); err != nil {
			return err
		}
		if err := tx.Where("recipe_id = ?", recipe.ID).Delete(&dbmodel.RecipeIngredient{}).Error; err != nil {
			return err
		}
		if err := tx.Where("recipe_id = ?", recipe.ID).Delete(&dbmodel.RecipeStep{}).Error; err != nil {
			return err
		}
		return tx.Delete(&dbmodel.Recipe{}, recipe.ID).Error
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
		"entity":  "recipe",
		"id":      recipe.ID,
	})
}

func createRecipe(user dbmodel.User, relation dbmodel.UserRelation, req createRecipeRequest) (*dbmodel.Recipe, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, recipeValidationError{message: "title is required"}
	}
	if len(req.Ingredients) == 0 {
		return nil, recipeValidationError{message: "at least one ingredient is required"}
	}
	if len(req.Steps) == 0 {
		return nil, recipeValidationError{message: "at least one step is required"}
	}

	recipe := &dbmodel.Recipe{
		ObjectBase: dbmodel.ObjectBase{
			CreatedBy: &user,
			UpdatedBy: &user,
		},
		RelationID:  relation.ID,
		UserID:      user.ID,
		Title:       title,
		Description: strings.TrimSpace(req.Description),
	}

	if err := database.Connection.Transaction(func(tx *gorm.DB) error {
		if err := recipe.Create(tx); err != nil {
			return err
		}
		return replaceRecipeDetails(tx, recipe.ID, req.Ingredients, req.Steps)
	}); err != nil {
		return nil, err
	}

	if err := recipe.GetWithDetails(database.Connection); err != nil {
		return nil, err
	}
	return recipe, nil
}

func applyRecipeUpdates(recipe *dbmodel.Recipe, req updateRecipeRequest, user dbmodel.User) error {
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return recipeValidationError{message: "title cannot be empty"}
		}
		recipe.Title = title
	}
	if req.Description != nil {
		recipe.Description = strings.TrimSpace(*req.Description)
	}
	recipe.UpdatedBy = &user

	if err := database.Connection.Transaction(func(tx *gorm.DB) error {
		if err := recipe.Update(tx); err != nil {
			return err
		}

		ingredients := recipeIngredientsToInputs(recipe.Ingredients)
		if req.Ingredients != nil {
			ingredients = *req.Ingredients
		}
		steps := recipeStepsToInputs(recipe.Steps)
		if req.Steps != nil {
			steps = *req.Steps
		}

		return replaceRecipeDetails(tx, recipe.ID, ingredients, steps)
	}); err != nil {
		return err
	}

	return recipe.GetWithDetails(database.Connection)
}

func replaceRecipeDetails(tx *gorm.DB, recipeID uint, ingredients []recipeIngredientInput, steps []recipeStepInput) error {
	if len(ingredients) == 0 {
		return recipeValidationError{message: "at least one ingredient is required"}
	}
	if len(steps) == 0 {
		return recipeValidationError{message: "at least one step is required"}
	}

	if err := tx.Where("recipe_id = ?", recipeID).Delete(&dbmodel.RecipeIngredient{}).Error; err != nil {
		return err
	}
	if err := tx.Where("recipe_id = ?", recipeID).Delete(&dbmodel.RecipeStep{}).Error; err != nil {
		return err
	}

	for index, item := range ingredients {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return recipeValidationError{message: "ingredient name is required"}
		}
		if err := tx.Create(&dbmodel.RecipeIngredient{
			RecipeID:  recipeID,
			SortOrder: index + 1,
			Name:      name,
			Quantity:  strings.TrimSpace(item.Quantity),
			Notes:     strings.TrimSpace(item.Notes),
		}).Error; err != nil {
			return err
		}
	}

	for index, item := range steps {
		instruction := strings.TrimSpace(item.Instruction)
		if instruction == "" {
			return recipeValidationError{message: "step instruction is required"}
		}
		if err := tx.Create(&dbmodel.RecipeStep{
			RecipeID:    recipeID,
			SortOrder:   index + 1,
			Instruction: instruction,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

func getRecipeFromRequest(r *http.Request) (*dbmodel.Recipe, error) {
	recipeID, err := strconv.Atoi(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil || recipeID <= 0 {
		return nil, recipeValidationError{message: "invalid recipe id"}
	}

	recipe := &dbmodel.Recipe{}
	recipe.ID = uint(recipeID)
	if err := recipe.GetWithDetails(database.Connection); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, recipeValidationError{message: "recipe not found"}
		}
		return nil, err
	}

	return recipe, nil
}

func buildRecipeSummaryResponse(recipe dbmodel.Recipe) RecipeSummaryResponse {
	return RecipeSummaryResponse{
		ID:              recipe.ID,
		UserID:          recipe.UserID,
		Title:           recipe.Title,
		Description:     recipe.Description,
		IngredientCount: len(recipe.Ingredients),
		StepCount:       len(recipe.Steps),
		CreatedAt:       recipe.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       recipe.UpdatedAt.Format(time.RFC3339),
	}
}

func buildRecipeDetailResponse(recipe dbmodel.Recipe) RecipeDetailResponse {
	response := RecipeDetailResponse{
		RecipeSummaryResponse: buildRecipeSummaryResponse(recipe),
		Ingredients:           make([]RecipeIngredientResponse, 0, len(recipe.Ingredients)),
		Steps:                 make([]RecipeStepResponse, 0, len(recipe.Steps)),
	}

	for _, item := range recipe.Ingredients {
		response.Ingredients = append(response.Ingredients, RecipeIngredientResponse{
			ID:        item.ID,
			SortOrder: item.SortOrder,
			Name:      item.Name,
			Quantity:  item.Quantity,
			Notes:     item.Notes,
		})
	}
	for _, item := range recipe.Steps {
		response.Steps = append(response.Steps, RecipeStepResponse{
			ID:          item.ID,
			SortOrder:   item.SortOrder,
			Instruction: item.Instruction,
		})
	}

	return response
}

func recipeIngredientsToInputs(items []dbmodel.RecipeIngredient) []recipeIngredientInput {
	output := make([]recipeIngredientInput, 0, len(items))
	for _, item := range items {
		output = append(output, recipeIngredientInput{
			Name:     item.Name,
			Quantity: item.Quantity,
			Notes:    item.Notes,
		})
	}
	return output
}

func recipeStepsToInputs(items []dbmodel.RecipeStep) []recipeStepInput {
	output := make([]recipeStepInput, 0, len(items))
	for _, item := range items {
		output = append(output, recipeStepInput{
			Instruction: item.Instruction,
		})
	}
	return output
}

func writeRecipeError(w http.ResponseWriter, err error) {
	var validation recipeValidationError
	if errors.As(err, &validation) {
		message := validation.Error()
		if message == "recipe not found" {
			http.Error(w, message, http.StatusNotFound)
			return
		}
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
