package service

import (
	"errors"
	"testing"

	"mapmarker/backend/database/dbmodel"
)

func TestNotifyMarkerCreatorDoesNotPropagateNotificationFailure(t *testing.T) {
	original := markerCreatedNotificationFn
	t.Cleanup(func() { markerCreatedNotificationFn = original })

	called := false
	markerCreatedNotificationFn = func(marker dbmodel.Marker, user dbmodel.User) error {
		called = true
		if marker.ID != 7 || user.Username != "walter" {
			t.Fatalf("unexpected notification input: marker=%+v user=%+v", marker, user)
		}
		return errors.New("Telegram unavailable")
	}

	notifyMarkerCreator(&dbmodel.Marker{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 7}}}, dbmodel.User{Username: "walter"})
	if !called {
		t.Fatal("expected notification attempt")
	}
}
