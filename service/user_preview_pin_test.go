package service

import (
	"errors"
	"testing"

	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
)

func resetUserPreviewPinHooks() {
	getUserPreviewPinSelectionFn = GetUserPreviewPinSelection
	getDefaultPinByLabelFn = GetDefaultPinByLabel
	getPinByIDFn = func(pinID uint) (*dbmodel.Pin, error) {
		var pin dbmodel.Pin
		pin.ID = pinID
		if err := pin.GetById(database.Connection); err != nil {
			return nil, err
		}
		return &pin, nil
	}
}

func TestResolveUserPreviewPinByUsernameUsesSelectedPin(t *testing.T) {
	resetUserPreviewPinHooks()
	defer resetUserPreviewPinHooks()

	selectedPinID := uint(11)
	selectedPin := &dbmodel.Pin{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: selectedPinID}}, Label: "selected", ImagePath: "/uploads/pins/selected.png"}
	getUserPreviewPinSelectionFn = func(username string) (*dbmodel.User, *dbmodel.UserPreference, error) {
		return &dbmodel.User{Username: username}, &dbmodel.UserPreference{PreviewPinID: &selectedPinID, PreviewPin: selectedPin}, nil
	}
	getDefaultPinByLabelFn = func(label string) (*dbmodel.DefaultValue, error) {
		t.Fatalf("default pin should not be queried when selected pin exists")
		return nil, nil
	}

	user, pin, err := ResolveUserPreviewPinByUsername("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil || user.Username != "alice" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if pin == nil || pin.ID != selectedPinID {
		t.Fatalf("expected selected pin id %d, got %+v", selectedPinID, pin)
	}
}

func TestResolveUserPreviewPinByUsernameFallsBackToDefault(t *testing.T) {
	resetUserPreviewPinHooks()
	defer resetUserPreviewPinHooks()

	defaultPinID := uint(21)
	defaultPin := &dbmodel.Pin{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: defaultPinID}}, Label: "default", ImagePath: "/uploads/pins/default.png"}
	getUserPreviewPinSelectionFn = func(username string) (*dbmodel.User, *dbmodel.UserPreference, error) {
		return &dbmodel.User{Username: username}, nil, nil
	}
	getDefaultPinByLabelFn = func(label string) (*dbmodel.DefaultValue, error) {
		return &dbmodel.DefaultValue{PinId: &defaultPinID, PinType: defaultPin}, nil
	}

	user, pin, err := ResolveUserPreviewPinByUsername("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil || user.Username != "alice" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if pin == nil || pin.ID != defaultPinID {
		t.Fatalf("expected default pin id %d, got %+v", defaultPinID, pin)
	}
}

func TestResolveUserPreviewPinByUsernameFailsWithoutSelectedOrDefault(t *testing.T) {
	resetUserPreviewPinHooks()
	defer resetUserPreviewPinHooks()

	getUserPreviewPinSelectionFn = func(username string) (*dbmodel.User, *dbmodel.UserPreference, error) {
		return &dbmodel.User{Username: username}, nil, nil
	}
	getDefaultPinByLabelFn = func(label string) (*dbmodel.DefaultValue, error) {
		return &dbmodel.DefaultValue{}, nil
	}

	_, _, err := ResolveUserPreviewPinByUsername("alice")
	if !errors.Is(err, ErrPreviewPinSelectionRequired) {
		t.Fatalf("expected ErrPreviewPinSelectionRequired, got %v", err)
	}
}
