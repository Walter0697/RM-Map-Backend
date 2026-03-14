package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
)

func resetHistoryMarkerPreviewHooks() {
	historyMarkerPreviewCurrentUserFn = currentUserFromRequest
	historyMarkerPreviewCurrentRelationFn = GetCurrentRelation
	historyMarkerPreviewGetMarkerFn = func(markerID uint) (*dbmodel.Marker, error) {
		var marker dbmodel.Marker
		marker.ID = markerID
		if err := marker.GetById(database.Connection); err != nil {
			return nil, err
		}
		return &marker, nil
	}
	historyMarkerPreviewResolvePinFn = ResolveUserPreviewPinByUsername
	historyMarkerPreviewGenerateFn = GenerateStaticMapPreviewByUsername
	historyMarkerPreviewStorageDirFn = func() string {
		return filepath.Join(constant.BasePath, strings.TrimPrefix(constant.PreviewImagePath, "/"))
	}
}

func TestBuildHistoryMarkerPreviewKeyNormalizesCoordinatePrecision(t *testing.T) {
	pin := &dbmodel.Pin{ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 9}}, ImagePath: "/pins/p1.png"}
	first := buildHistoryMarkerPreviewKey(dbmodel.Marker{Latitude: 22.3000001, Longitude: 114.2000001, Type: "food"}, historyMarkerPreviewProfileList, pin)
	second := buildHistoryMarkerPreviewKey(dbmodel.Marker{Latitude: 22.3000004, Longitude: 114.2000004, Type: "food"}, historyMarkerPreviewProfileList, pin)

	if first != second {
		t.Fatalf("expected normalized preview keys to match, got %s and %s", first, second)
	}
}

func TestHistoryMarkerPreviewHandlerRequiresAuthentication(t *testing.T) {
	resetHistoryMarkerPreviewHooks()
	defer resetHistoryMarkerPreviewHooks()

	historyMarkerPreviewCurrentUserFn = func(r *http.Request) *dbmodel.User { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/settings/history-marker-preview?marker_id=12", nil)
	HistoryMarkerPreviewHandler(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestHistoryMarkerPreviewHandlerSkipsProviderForInvalidCoordinates(t *testing.T) {
	resetHistoryMarkerPreviewHooks()
	defer resetHistoryMarkerPreviewHooks()

	historyMarkerPreviewCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{Username: "alice"}
	}
	historyMarkerPreviewCurrentRelationFn = func(user dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 7}}, nil
	}
	historyMarkerPreviewGetMarkerFn = func(markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 7,
			Latitude:   0,
			Longitude:  0,
			Type:       "food",
		}, nil
	}

	called := false
	historyMarkerPreviewGenerateFn = func(username string, markerTypeName string, lat float64, lon float64) (*staticPreviewResult, error) {
		called = true
		return nil, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/settings/history-marker-preview?marker_id=12", nil)
	HistoryMarkerPreviewHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if called {
		t.Fatalf("expected preview generation to be skipped for invalid coordinates")
	}
}

func TestHistoryMarkerPreviewHandlerReturnsCacheHitWithoutGenerating(t *testing.T) {
	resetHistoryMarkerPreviewHooks()
	defer resetHistoryMarkerPreviewHooks()

	tempDir := t.TempDir()
	historyMarkerPreviewStorageDirFn = func() string { return tempDir }
	historyMarkerPreviewCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{Username: "alice"}
	}
	historyMarkerPreviewCurrentRelationFn = func(user dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 7}}, nil
	}
	historyMarkerPreviewGetMarkerFn = func(markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 7,
			Latitude:   22.3,
			Longitude:  114.2,
			Type:       "food",
		}, nil
	}
	historyMarkerPreviewResolvePinFn = func(username string) (*dbmodel.User, *dbmodel.Pin, error) {
		return &dbmodel.User{Username: username}, &dbmodel.Pin{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 9}},
			ImagePath:  "/pins/p1.png",
		}, nil
	}

	cacheKey := buildHistoryMarkerPreviewKey(dbmodel.Marker{
		Latitude:  22.3,
		Longitude: 114.2,
		Type:      "food",
	}, historyMarkerPreviewProfileList, &dbmodel.Pin{
		ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 9}},
		ImagePath:  "/pins/p1.png",
	})
	cachePath := filepath.Join(tempDir, "history-marker-"+cacheKey+".png")
	if err := os.WriteFile(cachePath, []byte("cached"), 0o644); err != nil {
		t.Fatalf("failed to seed cache: %v", err)
	}

	called := false
	historyMarkerPreviewGenerateFn = func(username string, markerTypeName string, lat float64, lon float64) (*staticPreviewResult, error) {
		called = true
		return nil, nil
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/settings/history-marker-preview?marker_id=12", nil)
	HistoryMarkerPreviewHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if called {
		t.Fatalf("expected cache hit to avoid preview generation")
	}
	if !strings.Contains(recorder.Body.String(), `"cache_hit":true`) {
		t.Fatalf("expected cache_hit true payload, got %s", recorder.Body.String())
	}
}

func TestHistoryMarkerPreviewHandlerReturnsFallbackOnProviderFailure(t *testing.T) {
	resetHistoryMarkerPreviewHooks()
	defer resetHistoryMarkerPreviewHooks()

	historyMarkerPreviewStorageDirFn = func() string { return t.TempDir() }
	historyMarkerPreviewCurrentUserFn = func(r *http.Request) *dbmodel.User {
		return &dbmodel.User{Username: "alice"}
	}
	historyMarkerPreviewCurrentRelationFn = func(user dbmodel.User) (*dbmodel.UserRelation, error) {
		return &dbmodel.UserRelation{BaseModel: dbmodel.BaseModel{ID: 7}}, nil
	}
	historyMarkerPreviewGetMarkerFn = func(markerID uint) (*dbmodel.Marker, error) {
		return &dbmodel.Marker{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: markerID}},
			RelationId: 7,
			Latitude:   22.3,
			Longitude:  114.2,
			Type:       "food",
		}, nil
	}
	historyMarkerPreviewResolvePinFn = func(username string) (*dbmodel.User, *dbmodel.Pin, error) {
		return &dbmodel.User{Username: username}, &dbmodel.Pin{
			ObjectBase: dbmodel.ObjectBase{BaseModel: dbmodel.BaseModel{ID: 9}},
			ImagePath:  "/pins/p1.png",
		}, nil
	}
	historyMarkerPreviewGenerateFn = func(username string, markerTypeName string, lat float64, lon float64) (*staticPreviewResult, error) {
		return nil, ErrTomTomStaticMap
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/settings/history-marker-preview?marker_id=12", nil)
	HistoryMarkerPreviewHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"state":"fallback"`) {
		t.Fatalf("expected fallback payload, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"fallback_reason":"tomtom_dependency_failure"`) {
		t.Fatalf("expected provider failure fallback, got %s", recorder.Body.String())
	}
}
