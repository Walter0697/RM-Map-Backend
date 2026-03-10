package service

import (
	"encoding/json"
	"errors"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
)

type upsertPinGroupRequest struct {
	Name  string `json:"name"`
	IsNew bool   `json:"is_new"`
}

type updatePinAssignmentsRequest struct {
	GroupIDs []uint `json:"group_ids"`
}

type pinGroupResponse struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	IsNew bool   `json:"is_new"`
}

type pinSelectionResponse struct {
	ID          uint     `json:"id"`
	Label       string   `json:"label"`
	ImagePath   string   `json:"image_path"`
	DisplayPath string   `json:"display_path"`
	GroupIDs    []uint   `json:"group_ids"`
	GroupNames  []string `json:"group_names"`
	GroupIsNew  []bool   `json:"group_is_new"`
}

type pinSelectionGroupSection struct {
	GroupID   *uint                  `json:"group_id"`
	GroupName string                 `json:"group_name"`
	IsNew     bool                   `json:"is_new"`
	Pins      []pinSelectionResponse `json:"pins"`
}

type settingsPinsResponse struct {
	Pins   []pinSelectionResponse     `json:"pins"`
	Groups []pinSelectionGroupSection `json:"groups"`
}

var requireAdminUserFn = requireAdminUser
var requireUserOrAdminFn = requireUserOrAdmin
var listPinGroupsFn = ListPinGroups
var createPinGroupFn = CreatePinGroup
var updatePinGroupFn = UpdatePinGroup
var deletePinGroupFn = DeletePinGroup
var getPinGroupAssignmentsFn = GetPinGroupAssignments
var setPinGroupAssignmentsFn = SetPinGroupAssignments
var getAllPinsFn = GetAllPin

func mapPinGroupResponse(groups []dbmodel.PinGroup) []pinGroupResponse {
	out := make([]pinGroupResponse, 0, len(groups))
	for _, group := range groups {
		out = append(out, pinGroupResponse{
			ID:    group.ID,
			Name:  group.Name,
			IsNew: group.IsNew,
		})
	}
	return out
}

func mapPinSelection(pin dbmodel.Pin) pinSelectionResponse {
	type groupPair struct {
		ID    uint
		Name  string
		IsNew bool
	}
	groupPairs := make([]groupPair, 0, len(pin.Groups))
	for _, group := range pin.Groups {
		groupPairs = append(groupPairs, groupPair{
			ID:    group.ID,
			Name:  group.Name,
			IsNew: group.IsNew,
		})
	}
	sort.Slice(groupPairs, func(i, j int) bool {
		return strings.ToLower(groupPairs[i].Name) < strings.ToLower(groupPairs[j].Name)
	})
	groupIDs := make([]uint, 0, len(groupPairs))
	groupNames := make([]string, 0, len(groupPairs))
	groupIsNew := make([]bool, 0, len(groupPairs))
	for _, pair := range groupPairs {
		groupIDs = append(groupIDs, pair.ID)
		groupNames = append(groupNames, pair.Name)
		groupIsNew = append(groupIsNew, pair.IsNew)
	}
	return pinSelectionResponse{
		ID:          pin.ID,
		Label:       pin.Label,
		ImagePath:   pin.ImagePath,
		DisplayPath: pin.DisplayPath,
		GroupIDs:    groupIDs,
		GroupNames:  groupNames,
		GroupIsNew:  groupIsNew,
	}
}

func buildGroupedSelectionSections(items []pinSelectionResponse) []pinSelectionGroupSection {
	byGroup := map[uint]pinSelectionGroupSection{}
	ungrouped := make([]pinSelectionResponse, 0)
	for _, item := range items {
		if len(item.GroupIDs) == 0 {
			ungrouped = append(ungrouped, item)
			continue
		}
		for index, groupID := range item.GroupIDs {
			name := ""
			if index < len(item.GroupNames) {
				name = item.GroupNames[index]
			}
			existing := byGroup[groupID]
			if existing.GroupID == nil {
				id := groupID
				existing.GroupID = &id
				existing.GroupName = name
				if index < len(item.GroupIsNew) {
					existing.IsNew = item.GroupIsNew[index]
				}
				existing.Pins = []pinSelectionResponse{}
			}
			existing.Pins = append(existing.Pins, item)
			byGroup[groupID] = existing
		}
	}

	sections := make([]pinSelectionGroupSection, 0, len(byGroup)+1)
	for _, section := range byGroup {
		sections = append(sections, section)
	}
	sort.Slice(sections, func(i, j int) bool {
		return strings.ToLower(sections[i].GroupName) < strings.ToLower(sections[j].GroupName)
	})
	if len(ungrouped) > 0 {
		sections = append(sections, pinSelectionGroupSection{
			GroupID:   nil,
			GroupName: "Ungrouped",
			Pins:      ungrouped,
		})
	}
	return sections
}

func requireAdminUser(w http.ResponseWriter, r *http.Request) (*dbmodel.User, bool) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return nil, false
	}
	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return nil, false
	}
	return user, true
}

func requireUserOrAdmin(w http.ResponseWriter, r *http.Request) (*dbmodel.User, bool) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return nil, false
	}
	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return nil, false
	}
	return user, true
}

func AdminListPinGroupsHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := requireAdminUserFn(w, r)
	if !ok {
		return
	}
	items, err := listPinGroupsFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, mapPinGroupResponse(items))
}

func AdminCreatePinGroupHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := requireAdminUserFn(w, r)
	if !ok {
		return
	}

	request := upsertPinGroupRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	group, err := createPinGroupFn(request.Name, request.IsNew, user)
	if err != nil {
		switch {
		case errors.Is(err, ErrPinGroupNameRequired):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, ErrPinGroupDuplicateName):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	respondJSON(w, http.StatusCreated, pinGroupResponse{ID: group.ID, Name: group.Name, IsNew: group.IsNew})
}

func AdminUpdatePinGroupHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := requireAdminUserFn(w, r)
	if !ok {
		return
	}

	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	groupID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || groupID == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	request := upsertPinGroupRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	group, updateErr := updatePinGroupFn(uint(groupID), request.Name, request.IsNew, user)
	if updateErr != nil {
		switch {
		case errors.Is(updateErr, ErrPinGroupNameRequired):
			http.Error(w, updateErr.Error(), http.StatusBadRequest)
		case errors.Is(updateErr, ErrPinGroupDuplicateName):
			http.Error(w, updateErr.Error(), http.StatusConflict)
		case errors.Is(updateErr, ErrPinGroupNotFound):
			http.Error(w, updateErr.Error(), http.StatusNotFound)
		default:
			http.Error(w, updateErr.Error(), http.StatusInternalServerError)
		}
		return
	}

	respondJSON(w, http.StatusOK, pinGroupResponse{ID: group.ID, Name: group.Name, IsNew: group.IsNew})
}

func AdminDeletePinGroupHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := requireAdminUserFn(w, r)
	if !ok {
		return
	}

	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	groupID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || groupID == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := deletePinGroupFn(uint(groupID)); err != nil {
		if errors.Is(err, ErrPinGroupNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func AdminGetPinGroupAssignmentsHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := requireAdminUserFn(w, r)
	if !ok {
		return
	}

	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	pinID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || pinID == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	groups, listErr := getPinGroupAssignmentsFn(uint(pinID))
	if listErr != nil {
		http.Error(w, listErr.Error(), http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, mapPinGroupResponse(groups))
}

func AdminUpdatePinGroupAssignmentsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := requireAdminUserFn(w, r)
	if !ok {
		return
	}

	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	pinID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || pinID == 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	request := updatePinAssignmentsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := setPinGroupAssignmentsFn(uint(pinID), request.GroupIDs, user); err != nil {
		switch {
		case errors.Is(err, ErrPinGroupInvalidAssignment):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, ErrPinGroupSingleAssignmentOnly):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	groups, listErr := getPinGroupAssignmentsFn(uint(pinID))
	if listErr != nil {
		http.Error(w, listErr.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, mapPinGroupResponse(groups))
}

func SettingsListPinsHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := requireUserOrAdminFn(w, r)
	if !ok {
		return
	}

	items, err := getAllPinsFn(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseItems := make([]pinSelectionResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, mapPinSelection(item))
	}

	respondJSON(w, http.StatusOK, settingsPinsResponse{
		Pins:   responseItems,
		Groups: buildGroupedSelectionSections(responseItems),
	})
}
