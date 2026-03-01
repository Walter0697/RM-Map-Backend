package service

import (
	"bytes"
	"encoding/json"
	"io"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
	"mapmarker/backend/initdb/initmodel"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/go-chi/chi"
)

type adminUpsertStationRequest struct {
	MapName    string  `json:"map_name"`
	Identifier string  `json:"identifier"`
	Label      string  `json:"label"`
	LocalName  string  `json:"local_name"`
	PhotoX     float64 `json:"photo_x"`
	PhotoY     float64 `json:"photo_y"`
	MapX       float64 `json:"map_x"`
	MapY       float64 `json:"map_y"`
	LineInfo   string  `json:"line_info"`
}

type adminStationLineItem struct {
	Name      string `json:"name"`
	LocalName string `json:"local_name"`
	Colour    string `json:"colour"`
	Position  int    `json:"position"`
}

type adminStationLineCatalogItem struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	LocalName string `json:"local_name"`
	Colour    string `json:"colour"`
}

type adminUpdateStationLinesRequest struct {
	MapName    string                 `json:"map_name"`
	Identifier string                 `json:"identifier"`
	Lines      []adminStationLineItem `json:"lines"`
}

type adminSaveStationLineCatalogRequest struct {
	MapName string                        `json:"map_name"`
	Lines   []adminStationLineCatalogItem `json:"lines"`
}

type stationMapAssetResponse struct {
	MapName   string `json:"map_name"`
	ImagePath string `json:"image_path"`
	ImageURL  string `json:"image_url"`
	IconPath  string `json:"icon_path"`
	IconURL   string `json:"icon_url"`
}

type adminTrainStationMapRequest struct {
	MapName     string `json:"map_name"`
	MapLabel    string `json:"map_label"`
	NewMapName  string `json:"new_map_name"`
	NewMapLabel string `json:"new_map_label"`
}

type adminImportStationJSONResponse struct {
	MapName          string  `json:"map_name"`
	ImportedStations int     `json:"imported_stations"`
	Version          float64 `json:"version"`
}

type adminStationResponse struct {
	Identifier string  `json:"identifier"`
	Label      string  `json:"label"`
	LocalName  string  `json:"local_name"`
	PhotoX     float64 `json:"photo_x"`
	PhotoY     float64 `json:"photo_y"`
	MapX       float64 `json:"map_x"`
	MapY       float64 `json:"map_y"`
	Active     bool    `json:"active"`
	MapName    string  `json:"map_name"`
	LineInfo   string  `json:"line_info"`
}

func convertAdminStationResponse(input dbmodel.TrainStation) adminStationResponse {
	return adminStationResponse{
		Identifier: input.Identifier,
		Label:      input.Label,
		LocalName:  input.StationLocalName,
		PhotoX:     input.PhotoX,
		PhotoY:     input.PhotoY,
		MapX:       input.MapX,
		MapY:       input.MapY,
		Active:     false,
		MapName:    input.MapName,
		LineInfo:   input.LineInfo,
	}
}

func requireAdmin(w http.ResponseWriter, r *http.Request) *dbmodel.User {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return nil
	}
	if err := helper.IsAuthorize(*user, helper.Admin); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return nil
	}
	return user
}

func AdminListStationsHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	mapName := strings.TrimSpace(r.URL.Query().Get("map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}
	if err := EnsureTrainStationMapExists(mapName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := GetAllTrainStationsForAdmin(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]adminStationResponse, 0, len(items))
	for _, item := range items {
		output := convertAdminStationResponse(item)
		result = append(result, output)
	}

	respondJSON(w, http.StatusOK, result)
}

func AdminUpsertStationHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	request := adminUpsertStationRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request.MapName = strings.TrimSpace(request.MapName)
	request.Identifier = strings.TrimSpace(request.Identifier)
	request.Label = strings.TrimSpace(request.Label)
	request.LocalName = strings.TrimSpace(request.LocalName)
	request.LineInfo = strings.TrimSpace(request.LineInfo)
	if request.MapName == "" || request.Identifier == "" || request.Label == "" {
		http.Error(w, "map_name, identifier, and label are required", http.StatusBadRequest)
		return
	}
	if err := EnsureTrainStationMapExists(request.MapName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.LineInfo == "" {
		request.LineInfo = "[]"
	}
	if !json.Valid([]byte(request.LineInfo)) {
		http.Error(w, "line_info must be valid JSON", http.StatusBadRequest)
		return
	}

	item, err := UpsertTrainStationByIdentifier(dbmodel.TrainStation{
		MapName:          request.MapName,
		Identifier:       request.Identifier,
		Label:            request.Label,
		StationLocalName: request.LocalName,
		PhotoX:           request.PhotoX,
		PhotoY:           request.PhotoY,
		MapX:             request.MapX,
		MapY:             request.MapY,
		LineInfo:         request.LineInfo,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	output := convertAdminStationResponse(*item)
	respondJSON(w, http.StatusOK, output)
}

func AdminUpdateStationLinesHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	request := adminUpdateStationLinesRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	request.MapName = strings.TrimSpace(request.MapName)
	request.Identifier = strings.TrimSpace(request.Identifier)
	if request.MapName == "" || request.Identifier == "" {
		http.Error(w, "map_name and identifier are required", http.StatusBadRequest)
		return
	}
	if err := EnsureTrainStationMapExists(request.MapName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lines := make([]initmodel.LineInfo, 0, len(request.Lines))
	for _, line := range request.Lines {
		lines = append(lines, initmodel.LineInfo{
			Name:      strings.TrimSpace(line.Name),
			LocalName: strings.TrimSpace(line.LocalName),
			Colour:    strings.TrimSpace(line.Colour),
			Position:  uint(line.Position),
		})
	}

	station, err := UpdateTrainStationLines(request.MapName, request.Identifier, lines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	output := convertAdminStationResponse(*station)
	respondJSON(w, http.StatusOK, output)
}

func AdminListStationLineCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	mapName := strings.TrimSpace(r.URL.Query().Get("map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}

	items, err := ListTrainStationLineCatalog(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func AdminSaveStationLineCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	request := adminSaveStationLineCatalogRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	request.MapName = strings.TrimSpace(request.MapName)
	if request.MapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}

	lines := make([]TrainStationLineCatalogItem, 0, len(request.Lines))
	for _, line := range request.Lines {
		lines = append(lines, TrainStationLineCatalogItem{
			ID:        line.ID,
			MapName:   request.MapName,
			Name:      strings.TrimSpace(line.Name),
			LocalName: strings.TrimSpace(line.LocalName),
			Colour:    strings.TrimSpace(line.Colour),
		})
	}

	items, err := SaveTrainStationLineCatalog(request.MapName, lines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func AdminExportStationJSONHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}

	payload, err := ExportTrainStationSeedJSON(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+strings.ToLower(mapName)+"-stations.json\"")
	_, _ = w.Write([]byte(payload))
}

func AdminImportStationJSONHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, stationMapMaxUploadSize+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(raw)) > stationMapMaxUploadSize {
		http.Error(w, "upload too large", http.StatusBadRequest)
		return
	}

	result, err := ImportTrainStationSeedJSON(mapName, raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, adminImportStationJSONResponse{
		MapName:          result.MapName,
		ImportedStations: result.ImportedStations,
		Version:          result.Version,
	})
}

func AdminUploadStationMapIconHandler(w http.ResponseWriter, r *http.Request) {
	user := requireAdmin(w, r)
	if user == nil {
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(12 * 1000 * 1000); err != nil {
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
	raw, err := io.ReadAll(io.LimitReader(file, stationMapMaxUploadSize+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	asset, err := UploadTrainStationMapIconAsset(mapName, &upload, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, stationMapAssetResponse{
		MapName:   asset.MapName,
		ImagePath: asset.ImagePath,
		ImageURL:  "/image" + asset.ImagePath,
		IconPath:  asset.IconPath,
		IconURL:   "/image" + asset.IconPath,
	})
}

func AdminGetStationMapAssetHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}
	if err := EnsureTrainStationMapExists(mapName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	asset, err := GetTrainStationMapAsset(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if asset == nil {
		respondJSON(w, http.StatusOK, nil)
		return
	}

	respondJSON(w, http.StatusOK, stationMapAssetResponse{
		MapName:   asset.MapName,
		ImagePath: asset.ImagePath,
		ImageURL:  "/image" + asset.ImagePath,
		IconPath:  asset.IconPath,
		IconURL:   "/image" + asset.IconPath,
	})
}

func GetStationMapAssetHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUserFromRequest(r)
	if user == nil {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return
	}
	if err := helper.IsAuthorize(*user, helper.User); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}
	if err := EnsureTrainStationMapExists(mapName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	asset, err := GetTrainStationMapAsset(mapName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if asset == nil {
		respondJSON(w, http.StatusOK, nil)
		return
	}

	respondJSON(w, http.StatusOK, stationMapAssetResponse{
		MapName:   asset.MapName,
		ImagePath: asset.ImagePath,
		ImageURL:  "/image" + asset.ImagePath,
		IconPath:  asset.IconPath,
		IconURL:   "/image" + asset.IconPath,
	})
}

func AdminUploadStationMapAssetHandler(w http.ResponseWriter, r *http.Request) {
	user := requireAdmin(w, r)
	if user == nil {
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}
	if err := EnsureTrainStationMapExists(mapName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(12 * 1000 * 1000); err != nil {
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
	raw, err := io.ReadAll(io.LimitReader(file, stationMapMaxUploadSize+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	asset, err := UploadTrainStationMapAsset(mapName, &upload, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, stationMapAssetResponse{
		MapName:   asset.MapName,
		ImagePath: asset.ImagePath,
		ImageURL:  "/image" + asset.ImagePath,
		IconPath:  asset.IconPath,
		IconURL:   "/image" + asset.IconPath,
	})
}

func AdminListTrainStationMapsHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	items, err := ListTrainStationMaps()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func AdminCreateTrainStationMapHandler(w http.ResponseWriter, r *http.Request) {
	user := requireAdmin(w, r)
	if user == nil {
		return
	}

	request := adminTrainStationMapRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	item, err := CreateTrainStationMap(request.MapName, request.MapLabel, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func AdminUpdateTrainStationMapHandler(w http.ResponseWriter, r *http.Request) {
	user := requireAdmin(w, r)
	if user == nil {
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}

	request := adminTrainStationMapRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	item, err := UpdateTrainStationMap(mapName, request.NewMapName, firstNonEmpty(request.NewMapLabel, request.MapLabel), user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func AdminDeleteTrainStationMapHandler(w http.ResponseWriter, r *http.Request) {
	if requireAdmin(w, r) == nil {
		return
	}

	mapName := strings.TrimSpace(chi.URLParam(r, "map_name"))
	if mapName == "" {
		http.Error(w, "map_name is required", http.StatusBadRequest)
		return
	}

	if err := RemoveTrainStationMap(mapName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func firstNonEmpty(candidates ...string) string {
	for _, value := range candidates {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
