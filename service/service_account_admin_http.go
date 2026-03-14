package service

import (
	"encoding/json"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
)

type adminServiceAccountUpsertRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Role         string `json:"role"`
	RelationID   *uint  `json:"relation_id,omitempty"`
	Active       *bool  `json:"active,omitempty"`
	ActingUserID *uint  `json:"acting_user_id,omitempty"`
}

type adminServiceAccountResponse struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Role           string `json:"role"`
	RelationID     uint   `json:"relation_id"`
	Active         bool   `json:"active"`
	ActingUserID   *uint  `json:"acting_user_id,omitempty"`
	ActingUsername string `json:"acting_username,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func serviceAccountToAdminResponse(item dbmodel.ServiceAccount) adminServiceAccountResponse {
	actingUsername := ""
	if item.ActingUser != nil {
		actingUsername = strings.TrimSpace(item.ActingUser.Username)
	}
	return adminServiceAccountResponse{
		ID:             item.ID,
		Name:           item.Name,
		Description:    item.Description,
		Role:           item.Role,
		RelationID:     item.RelationID,
		Active:         item.Active,
		ActingUserID:   item.ActingUserID,
		ActingUsername: actingUsername,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339),
	}
}

func AdminListServiceAccountsHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	items := make([]dbmodel.ServiceAccount, 0)
	if err := database.Connection.Preload("ActingUser").Order("name asc").Find(&items).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]adminServiceAccountResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, serviceAccountToAdminResponse(item))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"items": resp})
}

func AdminCreateServiceAccountHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	request := adminServiceAccountUpsertRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.Role = strings.TrimSpace(strings.ToLower(request.Role))
	if request.Name == "" || request.Role == "" || request.RelationID == nil || *request.RelationID == 0 {
		http.Error(w, "name, role, and relation_id are required", http.StatusBadRequest)
		return
	}
	if !helper.IsValidRoles(request.Role) {
		http.Error(w, "role is invalid", http.StatusBadRequest)
		return
	}
	relation := dbmodel.UserRelation{}
	relation.ID = *request.RelationID
	if err := relation.GetRelationById(database.Connection); err != nil {
		http.Error(w, "relation not found", http.StatusBadRequest)
		return
	}
	if request.ActingUserID != nil {
		actingUser := dbmodel.User{}
		actingUser.ID = *request.ActingUserID
		if err := actingUser.GetUserById(database.Connection); err != nil {
			http.Error(w, "acting user not found", http.StatusBadRequest)
			return
		}
	}

	active := true
	if request.Active != nil {
		active = *request.Active
	}
	item := dbmodel.ServiceAccount{
		Name:         request.Name,
		Description:  request.Description,
		Role:         request.Role,
		RelationID:   *request.RelationID,
		Active:       active,
		ActingUserID: request.ActingUserID,
	}
	if err := item.Create(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = item.GetByID(database.Connection)
	respondJSON(w, http.StatusCreated, serviceAccountToAdminResponse(item))
}

func AdminUpdateServiceAccountHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	idParam := strings.TrimSpace(chi.URLParam(r, "id"))
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil || id == 0 {
		http.Error(w, "invalid service account id", http.StatusBadRequest)
		return
	}

	request := adminServiceAccountUpsertRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	item := dbmodel.ServiceAccount{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: uint(id)}}}
	if err := item.GetByID(database.Connection); err != nil {
		http.Error(w, "service account not found", http.StatusNotFound)
		return
	}

	if name := strings.TrimSpace(request.Name); name != "" {
		item.Name = name
	}
	if request.Description != "" {
		item.Description = strings.TrimSpace(request.Description)
	}
	if request.Role != "" {
		role := strings.TrimSpace(strings.ToLower(request.Role))
		if !helper.IsValidRoles(role) {
			http.Error(w, "role is invalid", http.StatusBadRequest)
			return
		}
		item.Role = role
	}
	if request.RelationID != nil {
		if *request.RelationID == 0 {
			http.Error(w, "relation_id is invalid", http.StatusBadRequest)
			return
		}
		relation := dbmodel.UserRelation{}
		relation.ID = *request.RelationID
		if err := relation.GetRelationById(database.Connection); err != nil {
			http.Error(w, "relation not found", http.StatusBadRequest)
			return
		}
		item.RelationID = *request.RelationID
	}
	if request.Active != nil {
		item.Active = *request.Active
	}
	if request.ActingUserID != nil {
		actingUser := dbmodel.User{}
		actingUser.ID = *request.ActingUserID
		if err := actingUser.GetUserById(database.Connection); err != nil {
			http.Error(w, "acting user not found", http.StatusBadRequest)
			return
		}
		item.ActingUserID = request.ActingUserID
	}
	if err := item.Update(database.Connection); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = item.GetByID(database.Connection)
	respondJSON(w, http.StatusOK, serviceAccountToAdminResponse(item))
}
