package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mapmarker/backend/database/dbmodel"

	"github.com/go-chi/chi"
)

func resetPinGroupHTTPHooks() {
	requireAdminUserFn = requireAdminUser
	requireUserOrAdminFn = requireUserOrAdmin
	listPinGroupsFn = ListPinGroups
	createPinGroupFn = CreatePinGroup
	updatePinGroupFn = UpdatePinGroup
	deletePinGroupFn = DeletePinGroup
	getPinGroupAssignmentsFn = GetPinGroupAssignments
	setPinGroupAssignmentsFn = SetPinGroupAssignments
	getAllPinsFn = GetAllPin
}

func TestSettingsListPinsHandlerGroupedAndUngrouped(t *testing.T) {
	resetPinGroupHTTPHooks()
	defer resetPinGroupHTTPHooks()

	requireUserOrAdminFn = func(w http.ResponseWriter, r *http.Request) (*dbmodel.User, bool) {
		return &dbmodel.User{Username: "tester", Role: "user"}, true
	}
	getAllPinsFn = func(requested []string) ([]dbmodel.Pin, error) {
		return []dbmodel.Pin{
			{
				ObjectBase:  dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 11}},
				Label:       "Grouped Pin",
				ImagePath:   "/uploads/pins/a.png",
				DisplayPath: "/uploads/pins/a-display.png",
				Groups: []dbmodel.PinGroup{
					{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 21}}, Name: "Commuting"},
				},
			},
			{
				ObjectBase:  dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 12}},
				Label:       "Ungrouped Pin",
				ImagePath:   "/uploads/pins/b.png",
				DisplayPath: "/uploads/pins/b-display.png",
			},
		}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/settings/pins", nil)
	SettingsListPinsHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	response := settingsPinsResponse{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(response.Pins) != 2 {
		t.Fatalf("expected flat pins list length 2 for backward compatibility, got %d", len(response.Pins))
	}
	if len(response.Groups) < 2 {
		t.Fatalf("expected grouped sections including ungrouped fallback, got %d", len(response.Groups))
	}

	foundUngrouped := false
	for _, section := range response.Groups {
		if section.GroupName == "Ungrouped" {
			foundUngrouped = true
			if len(section.Pins) != 1 || section.Pins[0].Label != "Ungrouped Pin" {
				t.Fatalf("unexpected ungrouped section payload: %+v", section.Pins)
			}
		}
	}
	if !foundUngrouped {
		t.Fatalf("expected ungrouped section to be present")
	}
}

func TestAdminPinGroupFlowCreateAssignAndList(t *testing.T) {
	resetPinGroupHTTPHooks()
	defer resetPinGroupHTTPHooks()

	requireAdminUserFn = func(w http.ResponseWriter, r *http.Request) (*dbmodel.User, bool) {
		return &dbmodel.User{BaseModel: dbmodel.BaseModel{ID: 99}, Username: "admin", Role: "admin"}, true
	}
	requireUserOrAdminFn = func(w http.ResponseWriter, r *http.Request) (*dbmodel.User, bool) {
		return &dbmodel.User{Username: "tester", Role: "user"}, true
	}

	var createdGroup dbmodel.PinGroup
	assignments := map[uint][]uint{}

	createPinGroupFn = func(name string, actor *dbmodel.User) (*dbmodel.PinGroup, error) {
		createdGroup = dbmodel.PinGroup{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 301}},
			Name:       name,
		}
		return &createdGroup, nil
	}
	setPinGroupAssignmentsFn = func(pinID uint, groupIDs []uint, actor *dbmodel.User) error {
		assignments[pinID] = append([]uint{}, groupIDs...)
		return nil
	}
	getPinGroupAssignmentsFn = func(pinID uint) ([]dbmodel.PinGroup, error) {
		groupIDs := assignments[pinID]
		items := make([]dbmodel.PinGroup, 0, len(groupIDs))
		for _, id := range groupIDs {
			items = append(items, dbmodel.PinGroup{
				ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: id}},
				Name:       "Commuting",
			})
		}
		return items, nil
	}
	getAllPinsFn = func(requested []string) ([]dbmodel.Pin, error) {
		return []dbmodel.Pin{
			{
				ObjectBase:  dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 500}},
				Label:       "Station Pin",
				ImagePath:   "/uploads/pins/station.png",
				DisplayPath: "/uploads/pins/station-display.png",
				Groups: []dbmodel.PinGroup{
					{
						ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: createdGroup.ID}},
						Name:       createdGroup.Name,
					},
				},
			},
		}, nil
	}

	// Create group
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/admin/pin-groups", bytes.NewBufferString(`{"name":"Commuting"}`))
	AdminCreatePinGroupHandler(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d", createRecorder.Code)
	}

	// Assign group to pin
	assignRecorder := httptest.NewRecorder()
	assignRequest := httptest.NewRequest(http.MethodPut, "/admin/pins/500/groups", bytes.NewBufferString(`{"group_ids":[301]}`))
	assignRoute := chi.NewRouteContext()
	assignRoute.URLParams.Add("id", "500")
	assignRequest = assignRequest.WithContext(contextWithRoute(assignRequest, assignRoute))
	AdminUpdatePinGroupAssignmentsHandler(assignRecorder, assignRequest)
	if assignRecorder.Code != http.StatusOK {
		t.Fatalf("assignment expected 200, got %d", assignRecorder.Code)
	}

	// Verify grouped settings payload contains the assigned pin under the created group
	settingsRecorder := httptest.NewRecorder()
	settingsRequest := httptest.NewRequest(http.MethodGet, "/settings/pins", nil)
	SettingsListPinsHandler(settingsRecorder, settingsRequest)
	if settingsRecorder.Code != http.StatusOK {
		t.Fatalf("settings expected 200, got %d", settingsRecorder.Code)
	}

	response := settingsPinsResponse{}
	if err := json.Unmarshal(settingsRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse settings response: %v", err)
	}
	if len(response.Groups) == 0 {
		t.Fatalf("expected grouped sections")
	}
	if response.Groups[0].GroupName != "Commuting" {
		t.Fatalf("expected group Commuting, got %s", response.Groups[0].GroupName)
	}
}

func TestSettingsListPinsHandlerSingleLoadCall(t *testing.T) {
	resetPinGroupHTTPHooks()
	defer resetPinGroupHTTPHooks()

	requireUserOrAdminFn = func(w http.ResponseWriter, r *http.Request) (*dbmodel.User, bool) {
		return &dbmodel.User{Username: "tester", Role: "user"}, true
	}

	callCount := 0
	getAllPinsFn = func(requested []string) ([]dbmodel.Pin, error) {
		callCount++
		return []dbmodel.Pin{}, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/settings/pins", nil)
	SettingsListPinsHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if callCount != 1 {
		t.Fatalf("expected single pin query call, got %d", callCount)
	}
}

func contextWithRoute(request *http.Request, route *chi.Context) context.Context {
	return context.WithValue(request.Context(), chi.RouteCtxKey, route)
}
