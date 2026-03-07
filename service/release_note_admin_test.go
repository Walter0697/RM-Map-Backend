package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/go-chi/chi"
	"mapmarker/backend/database/dbmodel"
)

func TestBuildReleaseNoteModelValidatesSemver(t *testing.T) {
	releaseNoteLoadBaselineVersionFn = func() (string, error) {
		return "2.9.4", nil
	}
	t.Cleanup(func() {
		releaseNoteLoadBaselineVersionFn = loadReleaseNoteBaselineVersion
	})

	_, err := buildReleaseNoteModel(nil, adminReleaseNoteUpsertRequest{
		Title:         "Release",
		Version:       "2.9.4",
		Content:       "content",
		ContentFormat: "markdown",
		PublishState:  "draft",
	})
	if err == nil || !strings.Contains(err.Error(), "greater") {
		t.Fatalf("expected semver validation error, got %v", err)
	}

	note, err := buildReleaseNoteModel(nil, adminReleaseNoteUpsertRequest{
		Title:         "Release",
		Version:       "2.9.5",
		Content:       "# title\nline",
		ContentFormat: "markdown",
		PublishState:  "published",
	})
	if err != nil {
		t.Fatalf("expected valid model, got error %v", err)
	}
	if note.PublishState != releaseNoteStatePublished {
		t.Fatalf("expected published state, got %s", note.PublishState)
	}
	if note.PublishedAt == nil {
		t.Fatal("expected published_at to be set")
	}
}

func TestBuildReleaseNoteModelSanitizesHTML(t *testing.T) {
	releaseNoteLoadBaselineVersionFn = func() (string, error) {
		return "1.0.0", nil
	}
	t.Cleanup(func() {
		releaseNoteLoadBaselineVersionFn = loadReleaseNoteBaselineVersion
	})

	note, err := buildReleaseNoteModel(nil, adminReleaseNoteUpsertRequest{
		Title:         "Release",
		Version:       "1.0.1",
		Content:       `<p>Hello</p><script>alert(1)</script><a href="javascript:bad">x</a>`,
		ContentFormat: "html",
		PublishState:  "draft",
	})
	if err != nil {
		t.Fatalf("expected valid html model, got error %v", err)
	}
	if strings.Contains(strings.ToLower(note.SanitizedContent), "<script") {
		t.Fatalf("sanitizer should remove script tags: %s", note.SanitizedContent)
	}
	if strings.Contains(strings.ToLower(note.SanitizedContent), "javascript:") {
		t.Fatalf("sanitizer should remove javascript href: %s", note.SanitizedContent)
	}
}

func TestAdminReleaseNoteHandlersRequireAdmin(t *testing.T) {
	releaseNoteRequireAdminFn = func(w http.ResponseWriter, r *http.Request) *dbmodel.User {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return nil
	}
	t.Cleanup(func() {
		releaseNoteRequireAdminFn = requireAdmin
	})

	handlers := []http.HandlerFunc{
		AdminListReleaseNotesHandler,
		AdminCreateReleaseNoteHandler,
		AdminUploadReleaseNoteImageHandler,
	}

	for _, handler := range handlers {
		req := httptest.NewRequest(http.MethodGet, "/admin/release-notes", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized for %T, got %d", handler, rec.Code)
		}
	}
}

func TestAdminReleaseNoteCRUDHandlers(t *testing.T) {
	releaseNoteRequireAdminFn = func(w http.ResponseWriter, r *http.Request) *dbmodel.User {
		return &dbmodel.User{Role: "admin"}
	}
	releaseNoteLoadBaselineVersionFn = func() (string, error) { return "1.0.0", nil }
	t.Cleanup(func() {
		releaseNoteRequireAdminFn = requireAdmin
		releaseNoteLoadBaselineVersionFn = loadReleaseNoteBaselineVersion
		releaseNoteListForAdminFn = listReleaseNotesForAdmin
		releaseNoteGetByIDFn = getReleaseNoteByID
		releaseNoteCreateFn = createReleaseNoteRecord
		releaseNoteSaveFn = saveReleaseNoteRecord
		releaseNoteDeleteFn = deleteReleaseNoteRecord
	})

	now := time.Now().UTC()
	releaseNoteListForAdminFn = func(publishedOnly bool) ([]dbmodel.ReleaseNote, error) {
		return []dbmodel.ReleaseNote{
			{
				BaseModel:        dbmodel.BaseModel{ID: 1, CreatedAt: now},
				UpdatedAt:        now,
				Title:            "Release A",
				Version:          "1.1.0",
				Content:          "hello",
				ContentFormat:    releaseNoteFormatMarkdown,
				PublishState:     releaseNoteStateDraft,
				SanitizedContent: "<p>hello</p>",
			},
		}, nil
	}

	listReq := httptest.NewRequest(http.MethodGet, "/admin/release-notes", nil)
	listRec := httptest.NewRecorder()
	AdminListReleaseNotesHandler(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listRec.Code)
	}
	var listPayload []releaseNoteResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("expected valid list json response, got %v", err)
	}
	if len(listPayload) != 1 || listPayload[0].Title != "Release A" {
		t.Fatalf("unexpected list payload: %+v", listPayload)
	}

	created := false
	releaseNoteCreateFn = func(note *dbmodel.ReleaseNote) error {
		created = true
		note.BaseModel.ID = 22
		return nil
	}
	createBody := `{"title":"Release B","version":"1.1.1","content":"line 1","content_format":"markdown","publish_state":"draft","image_refs":[]}`
	createReq := httptest.NewRequest(http.MethodPost, "/admin/release-notes", strings.NewReader(createBody))
	createRec := httptest.NewRecorder()
	AdminCreateReleaseNoteHandler(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	if !created {
		t.Fatal("expected create function to be called")
	}

	releaseNoteGetByIDFn = func(id uint) (*dbmodel.ReleaseNote, error) {
		return &dbmodel.ReleaseNote{
			BaseModel:     dbmodel.BaseModel{ID: id, CreatedAt: now},
			UpdatedAt:     now,
			Title:         "Release C",
			Version:       "1.1.2",
			Content:       "old",
			ContentFormat: releaseNoteFormatMarkdown,
			PublishState:  releaseNoteStateDraft,
			ImageRefs:     "[]",
		}, nil
	}
	updated := false
	releaseNoteSaveFn = func(note *dbmodel.ReleaseNote) error {
		updated = true
		return nil
	}

	updateBody := `{"title":"Release C2","version":"1.1.3","content":"updated","content_format":"html","publish_state":"published","image_refs":["/release_notes/a.png"]}`
	updateReq := newReleaseNoteRouteRequest(http.MethodPut, "/admin/release-notes/1", "1", strings.NewReader(updateBody))
	updateRec := httptest.NewRecorder()
	AdminUpdateReleaseNoteHandler(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}
	if !updated {
		t.Fatal("expected save function to be called")
	}

	deletedID := uint(0)
	releaseNoteDeleteFn = func(id uint) error {
		deletedID = id
		return nil
	}
	deleteReq := newReleaseNoteRouteRequest(http.MethodDelete, "/admin/release-notes/7", "7", nil)
	deleteRec := httptest.NewRecorder()
	AdminDeleteReleaseNoteHandler(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	if deletedID != 7 {
		t.Fatalf("expected delete id=7, got %d", deletedID)
	}
}

func TestUploadReleaseNoteImageValidation(t *testing.T) {
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(".", "uploads", "release_notes"))
	})

	bad := graphql.Upload{
		File:        bytes.NewReader([]byte("not-an-image")),
		Filename:    "bad.txt",
		Size:        int64(len("not-an-image")),
		ContentType: "text/plain",
	}
	if _, err := uploadReleaseNoteImage(&bad); err == nil {
		t.Fatal("expected non-image upload to be rejected")
	}

	good := graphql.Upload{
		File:        bytes.NewReader([]byte{0x89, 0x50, 0x4e, 0x47}),
		Filename:    "ok.png",
		Size:        4,
		ContentType: "image/png",
	}
	path, err := uploadReleaseNoteImage(&good)
	if err != nil {
		t.Fatalf("expected png upload accepted, got %v", err)
	}
	if !strings.HasPrefix(path, "/release_notes/") {
		t.Fatalf("unexpected upload path: %s", path)
	}
}

func newReleaseNoteRouteRequest(method string, path string, id string, body *strings.Reader) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, body)
	}
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
}
