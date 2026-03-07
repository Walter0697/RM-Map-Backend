package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/go-chi/chi"
	xhtml "golang.org/x/net/html"
	"gorm.io/gorm"
)

const (
	releaseNoteStateDraft      = "draft"
	releaseNoteStatePublished  = "published"
	releaseNoteFormatMarkdown  = "markdown"
	releaseNoteFormatHTML      = "html"
	releaseNoteNotesFormatJSON = "json"
	releaseNoteNotesFormatMD   = "md"
	releaseNoteImageMaxSize    = int64(5 * 1000 * 1000)
)

var (
	releaseNoteRequireAdminFn         = requireAdmin
	releaseNoteCurrentUserFromReqFn   = currentUserFromRequest
	releaseNoteLoadBaselineVersionFn  = loadReleaseNoteBaselineVersion
	releaseNoteUploadImageFn          = uploadReleaseNoteImage
	releaseNoteListForAdminFn         = listReleaseNotesForAdmin
	releaseNoteListPublishedFn        = listPublishedReleaseNotes
	releaseNoteGetByIDFn              = getReleaseNoteByID
	releaseNoteCreateFn               = createReleaseNoteRecord
	releaseNoteSaveFn                 = saveReleaseNoteRecord
	releaseNoteDeleteFn               = deleteReleaseNoteRecord
	releaseNoteNowFn                  = time.Now
	releaseNoteParseMultipartFormSize = int64(6 * 1000 * 1000)
)

type adminReleaseNoteUpsertRequest struct {
	Title         string   `json:"title"`
	Version       string   `json:"version"`
	Content       string   `json:"content"`
	ContentFormat string   `json:"content_format"`
	NotesFormat   string   `json:"notes_format"`
	IconRef       string   `json:"icon_ref"`
	PublishState  string   `json:"publish_state"`
	ImageRefs     []string `json:"image_refs"`
}

type releaseNoteResponse struct {
	ID              uint       `json:"id"`
	Title           string     `json:"title"`
	Version         string     `json:"version"`
	Content         string     `json:"content"`
	ContentFormat   string     `json:"content_format"`
	NotesFormat     string     `json:"notes_format"`
	RenderedContent string     `json:"rendered_content"`
	PublishState    string     `json:"publish_state"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	ImageRefs       []string   `json:"image_refs"`
	ImageURLs       []string   `json:"image_urls"`
	IconRef         string     `json:"icon_ref,omitempty"`
	IconURL         string     `json:"icon_url,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type releaseNoteImageUploadResponse struct {
	Path string `json:"path"`
	URL  string `json:"url"`
}

func AdminListReleaseNotesHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	publishedOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("published_only")), "true")
	items, err := releaseNoteListForAdminFn(publishedOnly)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]releaseNoteResponse, 0, len(items))
	for _, item := range items {
		result = append(result, convertReleaseNoteResponse(item))
	}
	respondJSON(w, http.StatusOK, result)
}

func AdminGetReleaseNoteHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	note, err := getReleaseNoteByIDParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, convertReleaseNoteResponse(*note))
}

func AdminCreateReleaseNoteHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	request := adminReleaseNoteUpsertRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	note, err := buildReleaseNoteModel(nil, request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := releaseNoteCreateFn(note); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, convertReleaseNoteResponse(*note))
}

func AdminUpdateReleaseNoteHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	existing, err := getReleaseNoteByIDParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request := adminReleaseNoteUpsertRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	note, err := buildReleaseNoteModel(existing, request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := releaseNoteSaveFn(note); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, convertReleaseNoteResponse(*note))
}

func AdminDeleteReleaseNoteHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	note, err := getReleaseNoteByIDParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := releaseNoteDeleteFn(note.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

func AdminPublishReleaseNoteHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	note, err := getReleaseNoteByIDParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := releaseNoteNowFn().UTC()
	note.PublishState = releaseNoteStatePublished
	note.PublishedAt = &now
	if err := releaseNoteSaveFn(note); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, convertReleaseNoteResponse(*note))
}

func AdminUnpublishReleaseNoteHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	note, err := getReleaseNoteByIDParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	note.PublishState = releaseNoteStateDraft
	note.PublishedAt = nil
	if err := releaseNoteSaveFn(note); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, convertReleaseNoteResponse(*note))
}

func AdminUploadReleaseNoteImageHandler(w http.ResponseWriter, r *http.Request) {
	if releaseNoteRequireAdminFn(w, r) == nil {
		return
	}

	if err := r.ParseMultipartForm(releaseNoteParseMultipartFormSize); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	raw, err := io.ReadAll(io.LimitReader(file, releaseNoteImageMaxSize+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(raw)) > releaseNoteImageMaxSize {
		http.Error(w, "upload too large", http.StatusBadRequest)
		return
	}
	if contentType == "" && len(raw) > 0 {
		contentType = http.DetectContentType(raw)
	}

	upload := graphql.Upload{
		File:        bytes.NewReader(raw),
		Filename:    fileHeader.Filename,
		Size:        int64(len(raw)),
		ContentType: contentType,
	}

	path, err := releaseNoteUploadImageFn(&upload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusCreated, releaseNoteImageUploadResponse{
		Path: path,
		URL:  "/image" + path,
	})
}

func SettingsListReleaseNotesHandler(w http.ResponseWriter, r *http.Request) {
	user := releaseNoteCurrentUserFromReqFn(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}

	items, err := releaseNoteListPublishedFn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]releaseNoteResponse, 0, len(items))
	for _, item := range items {
		result = append(result, convertReleaseNoteResponse(item))
	}
	respondJSON(w, http.StatusOK, result)
}

func getReleaseNoteByIDParam(r *http.Request) (*dbmodel.ReleaseNote, error) {
	idRaw := strings.TrimSpace(chi.URLParam(r, "id"))
	if idRaw == "" {
		return nil, errors.New("id is required")
	}
	id, err := strconv.Atoi(idRaw)
	if err != nil || id <= 0 {
		return nil, errors.New("id must be a positive integer")
	}
	item, err := releaseNoteGetByIDFn(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("release note not found")
		}
		return nil, err
	}
	return item, nil
}

func listReleaseNotesForAdmin(publishedOnly bool) ([]dbmodel.ReleaseNote, error) {
	_ = normalizeLegacyReleaseNotePublishState()
	items := []dbmodel.ReleaseNote{}
	query := database.Connection.Order("created_at desc")
	if publishedOnly {
		query = query.Where("publish_state = ?", releaseNoteStatePublished)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func listPublishedReleaseNotes() ([]dbmodel.ReleaseNote, error) {
	_ = normalizeLegacyReleaseNotePublishState()
	items := []dbmodel.ReleaseNote{}
	if err := database.Connection.Where("publish_state = ?", releaseNoteStatePublished).Order("published_at desc, created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func getReleaseNoteByID(id uint) (*dbmodel.ReleaseNote, error) {
	item := dbmodel.ReleaseNote{}
	if err := database.Connection.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func createReleaseNoteRecord(note *dbmodel.ReleaseNote) error {
	return database.Connection.Create(note).Error
}

func saveReleaseNoteRecord(note *dbmodel.ReleaseNote) error {
	return database.Connection.Save(note).Error
}

func deleteReleaseNoteRecord(id uint) error {
	return database.Connection.Delete(&dbmodel.ReleaseNote{}, id).Error
}

func normalizeLegacyReleaseNotePublishState() error {
	baselineVersion, err := releaseNoteLoadBaselineVersionFn()
	if err != nil {
		return err
	}
	items := []dbmodel.ReleaseNote{}
	if err := database.Connection.Where("(publish_state = '' OR publish_state IS NULL) AND published_at IS NULL").Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		version := normalizeSemver(strings.TrimSpace(item.Version))
		if version == "" {
			continue
		}
		if compareSemver(version, baselineVersion) <= 0 {
			item.PublishState = releaseNoteStatePublished
			timestamp := item.CreatedAt
			item.PublishedAt = &timestamp
			if saveErr := database.Connection.Save(&item).Error; saveErr != nil {
				return saveErr
			}
		}
	}
	return nil
}

func buildReleaseNoteModel(existing *dbmodel.ReleaseNote, request adminReleaseNoteUpsertRequest) (*dbmodel.ReleaseNote, error) {
	title := strings.TrimSpace(request.Title)
	version := normalizeSemver(strings.TrimSpace(request.Version))
	format := strings.ToLower(strings.TrimSpace(request.ContentFormat))
	notesFormat := strings.ToLower(strings.TrimSpace(request.NotesFormat))
	state := strings.ToLower(strings.TrimSpace(request.PublishState))
	content := strings.TrimSpace(request.Content)
	if version == "" {
		return nil, errors.New("version is required")
	}
	if format != releaseNoteFormatMarkdown && format != releaseNoteFormatHTML {
		return nil, errors.New("content_format must be markdown or html")
	}
	if content == "" {
		return nil, errors.New("content is required")
	}
	if state == "" {
		state = releaseNoteStateDraft
	}
	if state != releaseNoteStateDraft && state != releaseNoteStatePublished {
		return nil, errors.New("publish_state must be draft or published")
	}
	if notesFormat == "" {
		if existing != nil && strings.TrimSpace(existing.NotesFormat) != "" {
			notesFormat = strings.ToLower(strings.TrimSpace(existing.NotesFormat))
		} else {
			notesFormat = releaseNoteNotesFormatMD
		}
	}
	if notesFormat != releaseNoteNotesFormatJSON && notesFormat != releaseNoteNotesFormatMD {
		return nil, errors.New("notes_format must be json or md")
	}

	baselineVersion, err := releaseNoteLoadBaselineVersionFn()
	if err != nil {
		return nil, err
	}
	existingVersion := ""
	if existing != nil {
		existingVersion = normalizeSemver(strings.TrimSpace(existing.Version))
	}
	if existing == nil {
		if compareSemver(version, baselineVersion) != 0 {
			return nil, fmt.Errorf("new release note version must match current app version (%s)", baselineVersion)
		}
	} else if version != existingVersion {
		if compareSemver(version, baselineVersion) < 0 {
			return nil, fmt.Errorf("version must be equal to or greater than current app version (%s)", baselineVersion)
		}
	}

	contentForRender := content
	notesRaw := content
	if notesFormat == releaseNoteNotesFormatJSON {
		if json.Valid([]byte(content)) {
			contentForRender = jsonNotesToMarkdown(content)
			notesRaw = content
		} else {
			lines := strings.Split(content, "\n")
			encoded, err := json.Marshal(lines)
			if err != nil {
				return nil, errors.New("failed to encode notes json")
			}
			notesRaw = string(encoded)
			contentForRender = content
		}
	}

	sanitized := ""
	if format == releaseNoteFormatHTML {
		sanitized = sanitizeHTMLAllowlist(contentForRender)
	} else {
		sanitized = renderMarkdownAsSafeHTML(contentForRender)
	}
	if strings.TrimSpace(sanitized) == "" {
		return nil, errors.New("content is empty after sanitization")
	}

	imageRefs := normalizeImageRefs(request.ImageRefs)
	imageRefsRaw, _ := json.Marshal(imageRefs)
	iconRef := strings.TrimSpace(request.IconRef)
	publishedAt := (*time.Time)(nil)
	if state == releaseNoteStatePublished {
		now := releaseNoteNowFn().UTC()
		publishedAt = &now
	}

	target := &dbmodel.ReleaseNote{}
	if existing != nil {
		target = existing
	}
	target.Title = title
	target.Version = version
	target.Content = contentForRender
	target.ContentFormat = format
	target.NotesFormat = notesFormat
	target.SanitizedContent = sanitized
	target.PublishState = state
	target.PublishedAt = publishedAt
	target.ImageRefs = string(imageRefsRaw)
	if iconRef != "" {
		target.Icon = &iconRef
	} else {
		target.Icon = nil
	}

	target.Notes = notesRaw
	return target, nil
}

func normalizeImageRefs(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func convertReleaseNoteResponse(input dbmodel.ReleaseNote) releaseNoteResponse {
	imageRefs := []string{}
	if strings.TrimSpace(input.ImageRefs) != "" {
		_ = json.Unmarshal([]byte(input.ImageRefs), &imageRefs)
	}
	imageURLs := make([]string, 0, len(imageRefs))
	for _, item := range imageRefs {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			imageURLs = append(imageURLs, value)
		} else {
			imageURLs = append(imageURLs, "/image"+value)
		}
	}
	iconRef := ""
	iconURL := ""
	if input.Icon != nil && strings.TrimSpace(*input.Icon) != "" {
		iconRef = strings.TrimSpace(*input.Icon)
	}
	if iconRef != "" {
		if strings.HasPrefix(iconRef, "http://") || strings.HasPrefix(iconRef, "https://") || strings.HasPrefix(iconRef, "/") {
			iconURL = iconRef
			if strings.HasPrefix(iconRef, "/") {
				iconURL = "/image" + iconRef
			}
		} else {
			iconURL = ""
		}
	}

	content := input.Content
	notesFormat := firstNonEmpty(strings.TrimSpace(input.NotesFormat), releaseNoteNotesFormatJSON)
	if strings.TrimSpace(content) == "" {
		if notesFormat == releaseNoteNotesFormatJSON {
			content = jsonNotesToMarkdown(input.Notes)
		} else {
			content = input.Notes
		}
	}
	rendered := input.SanitizedContent
	if strings.TrimSpace(rendered) == "" {
		if strings.TrimSpace(input.ContentFormat) == releaseNoteFormatHTML {
			rendered = sanitizeHTMLAllowlist(content)
		} else {
			rendered = renderMarkdownAsSafeHTML(content)
		}
	}

	state := strings.TrimSpace(strings.ToLower(input.PublishState))
	if state == "" {
		state = releaseNoteStatePublished
	}

	return releaseNoteResponse{
		ID:              input.ID,
		Title:           input.Title,
		Version:         input.Version,
		Content:         content,
		ContentFormat:   firstNonEmpty(strings.TrimSpace(input.ContentFormat), releaseNoteFormatMarkdown),
		NotesFormat:     notesFormat,
		RenderedContent: rendered,
		PublishState:    state,
		PublishedAt:     input.PublishedAt,
		ImageRefs:       imageRefs,
		ImageURLs:       imageURLs,
		IconRef:         iconRef,
		IconURL:         iconURL,
		CreatedAt:       input.CreatedAt,
		UpdatedAt:       input.UpdatedAt,
	}
}

func loadReleaseNoteBaselineVersion() (string, error) {
	version := normalizeSemver(constant.AppVersion)
	if version == "" {
		return "0.0.0", nil
	}
	return version, nil
}

func normalizeSemver(input string) string {
	value := strings.TrimSpace(strings.ToLower(input))
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return ""
	}
	value = strings.Split(value, "+")[0]
	mainPart := strings.Split(value, "-")[0]
	segments := strings.Split(mainPart, ".")
	if len(segments) < 3 {
		return ""
	}
	for i := 0; i < 3; i++ {
		if _, err := strconv.Atoi(strings.TrimSpace(segments[i])); err != nil {
			return ""
		}
	}
	return value
}

func compareSemver(a, b string) int {
	pa := parseSemverComparable(normalizeSemver(a))
	pb := parseSemverComparable(normalizeSemver(b))
	for i := 0; i < 3; i++ {
		if pa.numbers[i] > pb.numbers[i] {
			return 1
		}
		if pa.numbers[i] < pb.numbers[i] {
			return -1
		}
	}
	if pa.preRelease == "" && pb.preRelease == "" {
		return 0
	}
	if pa.preRelease == "" {
		return 1
	}
	if pb.preRelease == "" {
		return -1
	}
	return comparePreRelease(pa.preRelease, pb.preRelease)
}

type semverComparable struct {
	numbers    [3]int
	preRelease string
}

func parseSemverComparable(value string) semverComparable {
	result := semverComparable{}
	parts := strings.SplitN(value, "-", 2)
	numbers := strings.Split(parts[0], ".")
	for i := 0; i < 3 && i < len(numbers); i++ {
		parsed, _ := strconv.Atoi(numbers[i])
		result.numbers[i] = parsed
	}
	if len(parts) == 2 {
		result.preRelease = parts[1]
	}
	return result
}

func comparePreRelease(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	max := len(partsA)
	if len(partsB) > max {
		max = len(partsB)
	}
	for i := 0; i < max; i++ {
		if i >= len(partsA) {
			return -1
		}
		if i >= len(partsB) {
			return 1
		}
		left := partsA[i]
		right := partsB[i]
		leftN, leftErr := strconv.Atoi(left)
		rightN, rightErr := strconv.Atoi(right)
		if leftErr == nil && rightErr == nil {
			if leftN > rightN {
				return 1
			}
			if leftN < rightN {
				return -1
			}
			continue
		}
		if leftErr == nil {
			return -1
		}
		if rightErr == nil {
			return 1
		}
		if left > right {
			return 1
		}
		if left < right {
			return -1
		}
	}
	return 0
}

func renderMarkdownAsSafeHTML(markdown string) string {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	builder := strings.Builder{}
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		builder.WriteString("<p>")
		builder.WriteString(stdhtml.EscapeString(value))
		builder.WriteString("</p>")
	}
	return builder.String()
}

func notesListToMarkdown(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	trimmed := make([]string, 0, len(notes))
	for _, note := range notes {
		value := strings.TrimSpace(note)
		if value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return strings.Join(trimmed, "\n")
}

func jsonNotesToMarkdown(raw string) string {
	parsed := []string{}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil && len(parsed) > 0 {
		return notesListToMarkdown(parsed)
	}
	return strings.TrimSpace(raw)
}

func sanitizeHTMLAllowlist(input string) string {
	allowTags := map[string]bool{
		"p": true, "br": true, "strong": true, "em": true, "u": true,
		"ul": true, "ol": true, "li": true, "blockquote": true,
		"a": true, "img": true, "code": true, "pre": true,
		"h1": true, "h2": true, "h3": true, "h4": true,
	}
	allowAttrs := map[string]map[string]bool{
		"a":   {"href": true, "title": true, "target": true, "rel": true},
		"img": {"src": true, "alt": true, "title": true},
	}
	tokenizer := xhtml.NewTokenizer(strings.NewReader(input))
	builder := strings.Builder{}
	for {
		tt := tokenizer.Next()
		switch tt {
		case xhtml.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return builder.String()
			}
			return builder.String()
		case xhtml.TextToken:
			builder.WriteString(stdhtml.EscapeString(string(tokenizer.Text())))
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			token := tokenizer.Token()
			tag := strings.ToLower(strings.TrimSpace(token.Data))
			if !allowTags[tag] {
				continue
			}
			builder.WriteString("<")
			builder.WriteString(tag)
			allowedAttr := allowAttrs[tag]
			for _, attr := range token.Attr {
				name := strings.ToLower(strings.TrimSpace(attr.Key))
				if strings.HasPrefix(name, "on") {
					continue
				}
				if len(allowedAttr) > 0 && !allowedAttr[name] {
					continue
				}
				value := strings.TrimSpace(attr.Val)
				if (name == "href" || name == "src") && strings.HasPrefix(strings.ToLower(value), "javascript:") {
					continue
				}
				if name == "src" {
					if !(strings.HasPrefix(value, "/") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")) {
						continue
					}
				}
				builder.WriteString(" ")
				builder.WriteString(name)
				builder.WriteString(`="`)
				builder.WriteString(stdhtml.EscapeString(value))
				builder.WriteString(`"`)
			}
			if tt == xhtml.SelfClosingTagToken {
				builder.WriteString("/>")
			} else {
				builder.WriteString(">")
			}
		case xhtml.EndTagToken:
			token := tokenizer.Token()
			tag := strings.ToLower(strings.TrimSpace(token.Data))
			if !allowTags[tag] {
				continue
			}
			builder.WriteString("</")
			builder.WriteString(tag)
			builder.WriteString(">")
		}
	}
}

func uploadReleaseNoteImage(upload *graphql.Upload) (string, error) {
	contentType := strings.TrimSpace(upload.ContentType)
	if !strings.HasPrefix(contentType, "image/") {
		return "", errors.New("file must be an image")
	}

	raw, err := io.ReadAll(io.LimitReader(upload.File, releaseNoteImageMaxSize+1))
	if err != nil {
		return "", err
	}
	if int64(len(raw)) > releaseNoteImageMaxSize {
		return "", errors.New("upload too large")
	}

	extension := sanitizeUploadExtension(upload)
	switch extension {
	case "jpg", "png", "gif", "webp":
	default:
		return "", errors.New("unsupported image type")
	}

	filename := constant.GetImageName(constant.ReleaseNoteImagePath, extension)
	targetPath := filepath.Join(constant.BasePath, filename)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, raw, 0o644); err != nil {
		return "", err
	}
	return filename, nil
}

func BackfillLegacyReleaseNotes() (int, error) {
	items := []dbmodel.ReleaseNote{}
	if err := database.Connection.Find(&items).Error; err != nil {
		return 0, err
	}

	updated := 0
	for _, item := range items {
		if strings.TrimSpace(item.Content) != "" && strings.TrimSpace(item.ContentFormat) != "" && strings.TrimSpace(item.NotesFormat) != "" {
			continue
		}
		item.Title = firstNonEmpty(strings.TrimSpace(item.Title), "Release "+strings.TrimSpace(item.Version))
		legacyNotesFormat := releaseNoteNotesFormatMD
		legacyList := []string{}
		if strings.HasPrefix(strings.TrimSpace(item.Notes), "[") {
			if err := json.Unmarshal([]byte(item.Notes), &legacyList); err == nil && len(legacyList) > 0 {
				legacyNotesFormat = releaseNoteNotesFormatJSON
			}
		}
		if strings.TrimSpace(item.Content) == "" {
			if legacyNotesFormat == releaseNoteNotesFormatJSON {
				item.Content = notesListToMarkdown(legacyList)
			} else {
				item.Content = strings.TrimSpace(item.Notes)
			}
		}
		item.Content = firstNonEmpty(strings.TrimSpace(item.Content), strings.TrimSpace(item.Notes))
		item.ContentFormat = firstNonEmpty(strings.TrimSpace(item.ContentFormat), releaseNoteFormatMarkdown)
		item.NotesFormat = firstNonEmpty(strings.TrimSpace(item.NotesFormat), legacyNotesFormat)
		if item.ContentFormat == releaseNoteFormatHTML {
			item.SanitizedContent = sanitizeHTMLAllowlist(item.Content)
		} else {
			item.SanitizedContent = renderMarkdownAsSafeHTML(item.Content)
		}
		if strings.TrimSpace(item.PublishState) == "" {
			item.PublishState = releaseNoteStatePublished
		}
		if item.PublishState == releaseNoteStatePublished && item.PublishedAt == nil {
			timestamp := item.CreatedAt
			item.PublishedAt = &timestamp
		}
		if strings.TrimSpace(item.ImageRefs) == "" && item.Icon != nil && strings.TrimSpace(*item.Icon) != "" {
			refsRaw, _ := json.Marshal([]string{strings.TrimSpace(*item.Icon)})
			item.ImageRefs = string(refsRaw)
		}
		if err := database.Connection.Save(&item).Error; err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
