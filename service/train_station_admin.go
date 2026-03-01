package service

import (
	"encoding/json"
	"fmt"
	"io"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/helper"
	"mapmarker/backend/initdb/initmodel"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"gorm.io/gorm"
)

const stationMapMaxUploadSize int64 = 10 * 1000 * 1000

type stationLineJoinRow struct {
	StationID uint   `gorm:"column:station_id"`
	Name      string `gorm:"column:name"`
	LocalName string `gorm:"column:local_name"`
	Colour    string `gorm:"column:colour"`
	Position  int    `gorm:"column:position"`
}

func stationLineKey(mapName, name, localName, colour string) string {
	return strings.TrimSpace(mapName) + "|" + strings.TrimSpace(name) + "|" + strings.TrimSpace(localName) + "|" + strings.TrimSpace(colour)
}

func parseLineInfoJSON(input string) ([]initmodel.LineInfo, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return []initmodel.LineInfo{}, nil
	}
	lines := make([]initmodel.LineInfo, 0)
	if err := json.Unmarshal([]byte(trimmed), &lines); err != nil {
		return nil, err
	}
	return lines, nil
}

func ensureTrainStationLineTx(tx *gorm.DB, mapName string, line initmodel.LineInfo) (*dbmodel.TrainStationLine, error) {
	name := strings.TrimSpace(line.Name)
	localName := strings.TrimSpace(line.LocalName)
	colour := strings.TrimSpace(line.Colour)

	var existing dbmodel.TrainStationLine
	err := tx.Where("map_name = ? AND name = ? AND local_name = ? AND colour = ?", mapName, name, localName, colour).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	created := dbmodel.TrainStationLine{
		MapName:   mapName,
		Name:      name,
		LocalName: localName,
		Colour:    colour,
	}
	if err := tx.Create(&created).Error; err != nil {
		return nil, err
	}
	return &created, nil
}

func replaceStationLineRelationsTx(tx *gorm.DB, station dbmodel.TrainStation, lines []initmodel.LineInfo) error {
	if err := tx.Unscoped().Where("station_id = ?", station.ID).Delete(&dbmodel.TrainStationStationLine{}).Error; err != nil {
		return err
	}

	cache := make(map[string]uint)
	for _, raw := range lines {
		key := stationLineKey(station.MapName, raw.Name, raw.LocalName, raw.Colour)
		lineID, ok := cache[key]
		if !ok {
			line, err := ensureTrainStationLineTx(tx, station.MapName, raw)
			if err != nil {
				return err
			}
			lineID = line.ID
			cache[key] = lineID
		}

		relation := dbmodel.TrainStationStationLine{
			StationID: station.ID,
			LineID:    lineID,
			Position:  int(raw.Position),
		}
		if err := tx.Create(&relation).Error; err != nil {
			return err
		}
	}

	return nil
}

func buildStationLineInfoMapTx(tx *gorm.DB, stationIDs []uint) (map[uint]string, error) {
	output := make(map[uint]string, len(stationIDs))
	if len(stationIDs) == 0 {
		return output, nil
	}

	rows := make([]stationLineJoinRow, 0)
	if err := tx.Table("train_station_station_lines tsl").
		Select("tsl.station_id, tsl.position, tsl2.name, tsl2.local_name, tsl2.colour").
		Joins("JOIN train_station_lines tsl2 ON tsl2.id = tsl.line_id AND tsl2.deleted_at IS NULL").
		Where("tsl.station_id IN ?", stationIDs).
		Order("tsl.station_id asc, tsl.position asc, tsl.id asc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	lineMap := make(map[uint][]initmodel.LineInfo)
	for _, row := range rows {
		lineMap[row.StationID] = append(lineMap[row.StationID], initmodel.LineInfo{
			Name:      row.Name,
			LocalName: row.LocalName,
			Colour:    row.Colour,
			Position:  uint(row.Position),
		})
	}

	for _, stationID := range stationIDs {
		lines := lineMap[stationID]
		if len(lines) == 0 {
			output[stationID] = "[]"
			continue
		}
		bytes, err := json.Marshal(lines)
		if err != nil {
			return nil, err
		}
		output[stationID] = string(bytes)
	}

	return output, nil
}

func HydrateTrainStationLineInfoTx(tx *gorm.DB, stations []dbmodel.TrainStation) error {
	stationIDs := make([]uint, 0, len(stations))
	for _, station := range stations {
		stationIDs = append(stationIDs, station.ID)
	}
	lineInfoMap, err := buildStationLineInfoMapTx(tx, stationIDs)
	if err != nil {
		return err
	}
	for i := range stations {
		if value, ok := lineInfoMap[stations[i].ID]; ok {
			stations[i].LineInfo = value
		} else if strings.TrimSpace(stations[i].LineInfo) == "" {
			stations[i].LineInfo = "[]"
		}
	}
	return nil
}

func HydrateTrainStationLineInfo(stations []dbmodel.TrainStation) error {
	return HydrateTrainStationLineInfoTx(database.Connection, stations)
}

func RebuildStationLineInfoForMapTx(tx *gorm.DB, mapName string) error {
	ids := make([]uint, 0)
	if err := tx.Model(&dbmodel.TrainStation{}).Where("map_name = ?", strings.TrimSpace(mapName)).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	infoMap, err := buildStationLineInfoMapTx(tx, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		value, ok := infoMap[id]
		if !ok {
			value = "[]"
		}
		if err := tx.Model(&dbmodel.TrainStation{}).Where("id = ?", id).Update("line_info", value).Error; err != nil {
			return err
		}
	}
	return nil
}

func sanitizeUploadExtension(upload *graphql.Upload) string {
	contentType := strings.TrimSpace(upload.ContentType)
	if contentType != "" {
		typeInfo := strings.Split(contentType, "/")
		if len(typeInfo) == 2 {
			extension := strings.ToLower(strings.TrimSpace(typeInfo[1]))
			extension = strings.TrimPrefix(extension, "x-")
			extension = strings.Split(extension, ";")[0]
			extension = strings.ReplaceAll(extension, "jpeg", "jpg")
			if extension != "" {
				return extension
			}
		}
	}

	fallback := strings.TrimPrefix(strings.ToLower(filepath.Ext(upload.Filename)), ".")
	if fallback == "jpeg" {
		return "jpg"
	}
	return fallback
}

func UploadTrainStationMapAsset(mapName string, upload *graphql.Upload, actor *dbmodel.User) (*dbmodel.TrainStationMap, error) {
	contentType := strings.TrimSpace(upload.ContentType)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, &helper.UploadFileNotImageError{}
	}

	raw, err := io.ReadAll(io.LimitReader(upload.File, stationMapMaxUploadSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > stationMapMaxUploadSize {
		return nil, &helper.UploadFileTooLargeError{}
	}

	extension := sanitizeUploadExtension(upload)
	if extension == "" {
		return nil, &helper.UploadFileNotImageError{}
	}

	filename := constant.GetImageName(constant.StationMapPath, extension)
	targetPath := filepath.Join(constant.BasePath, filename)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(targetPath, raw, 0o644); err != nil {
		return nil, err
	}

	tx := database.Connection.Begin()
	var existing dbmodel.TrainStationMap
	err = tx.Where("map_name = ?", mapName).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		return nil, err
	}

	asset := dbmodel.TrainStationMap{
		ObjectBase: dbmodel.ObjectBase{
			UpdatedBy: actor,
		},
		MapName:   mapName,
		ImagePath: filename,
	}
	if err == gorm.ErrRecordNotFound {
		asset.ObjectBase.CreatedBy = actor
		if createErr := tx.Create(&asset).Error; createErr != nil {
			tx.Rollback()
			return nil, createErr
		}
	} else {
		asset.ID = existing.ID
		asset.ObjectBase.CreatedUID = existing.CreatedUID
		asset.ObjectBase.CreatedBy = existing.CreatedBy
		asset.MapLabel = existing.MapLabel
		asset.IconPath = existing.IconPath
		if saveErr := tx.Save(&asset).Error; saveErr != nil {
			tx.Rollback()
			return nil, saveErr
		}
	}

	if commitErr := tx.Commit().Error; commitErr != nil {
		return nil, commitErr
	}

	if err == nil && existing.ImagePath != "" && existing.ImagePath != asset.ImagePath {
		_ = os.Remove(filepath.Join(constant.BasePath, existing.ImagePath))
	}

	return &asset, nil
}

func UploadTrainStationMapIconAsset(mapName string, upload *graphql.Upload, actor *dbmodel.User) (*dbmodel.TrainStationMap, error) {
	contentType := strings.TrimSpace(upload.ContentType)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, &helper.UploadFileNotImageError{}
	}

	raw, err := io.ReadAll(io.LimitReader(upload.File, stationMapMaxUploadSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > stationMapMaxUploadSize {
		return nil, &helper.UploadFileTooLargeError{}
	}

	extension := sanitizeUploadExtension(upload)
	if extension == "" {
		return nil, &helper.UploadFileNotImageError{}
	}

	filename := constant.GetImageName(constant.StationMapIconPath, extension)
	targetPath := filepath.Join(constant.BasePath, filename)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(targetPath, raw, 0o644); err != nil {
		return nil, err
	}

	tx := database.Connection.Begin()
	var existing dbmodel.TrainStationMap
	err = tx.Where("map_name = ?", strings.TrimSpace(mapName)).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		return nil, err
	}

	asset := dbmodel.TrainStationMap{
		ObjectBase: dbmodel.ObjectBase{
			UpdatedBy: actor,
		},
		MapName:   strings.TrimSpace(mapName),
		ImagePath: existing.ImagePath,
		MapLabel:  existing.MapLabel,
		IconPath:  filename,
	}
	if err == gorm.ErrRecordNotFound {
		asset.ObjectBase.CreatedBy = actor
		if strings.TrimSpace(asset.MapLabel) == "" {
			asset.MapLabel = asset.MapName
		}
		if createErr := tx.Create(&asset).Error; createErr != nil {
			tx.Rollback()
			return nil, createErr
		}
	} else {
		asset.ID = existing.ID
		asset.ObjectBase.CreatedUID = existing.CreatedUID
		asset.ObjectBase.CreatedBy = existing.CreatedBy
		if saveErr := tx.Save(&asset).Error; saveErr != nil {
			tx.Rollback()
			return nil, saveErr
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	oldPath := existing.IconPath
	if strings.TrimSpace(oldPath) != "" && oldPath != filename {
		_ = os.Remove(filepath.Join(constant.BasePath, oldPath))
	}

	return &asset, nil
}

func GetTrainStationMapAsset(mapName string) (*dbmodel.TrainStationMap, error) {
	var asset dbmodel.TrainStationMap
	asset.MapName = mapName
	if err := asset.GetByMapName(database.Connection); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &asset, nil
}

func GetAllTrainStationsForAdmin(mapName string) ([]dbmodel.TrainStation, error) {
	var stations []dbmodel.TrainStation
	query := database.Connection.Model(&dbmodel.TrainStation{})
	if strings.TrimSpace(mapName) != "" {
		query = query.Where("map_name = ?", mapName)
	}
	if err := query.Order("label asc").Find(&stations).Error; err != nil {
		return nil, err
	}
	if err := HydrateTrainStationLineInfo(stations); err != nil {
		return nil, err
	}
	return stations, nil
}

func UpsertTrainStationByIdentifier(station dbmodel.TrainStation) (*dbmodel.TrainStation, error) {
	station.Identifier = strings.TrimSpace(station.Identifier)
	station.MapName = strings.TrimSpace(station.MapName)
	if station.Identifier == "" || station.MapName == "" {
		return nil, gorm.ErrRecordNotFound
	}

	lines, err := parseLineInfoJSON(station.LineInfo)
	if err != nil {
		return nil, err
	}

	tx := database.Connection.Begin()
	var existing dbmodel.TrainStation
	err = tx.Where("identifier = ? AND map_name = ?", station.Identifier, station.MapName).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if createErr := station.Create(tx); createErr != nil {
				tx.Rollback()
				return nil, createErr
			}
		} else {
			tx.Rollback()
			return nil, err
		}
	} else {
		station.ID = existing.ID
		if saveErr := station.Update(tx); saveErr != nil {
			tx.Rollback()
			return nil, saveErr
		}
	}

	if err := replaceStationLineRelationsTx(tx, station, lines); err != nil {
		tx.Rollback()
		return nil, err
	}

	infoMap, err := buildStationLineInfoMapTx(tx, []uint{station.ID})
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if value, ok := infoMap[station.ID]; ok {
		station.LineInfo = value
	} else {
		station.LineInfo = "[]"
	}
	if err := tx.Model(&dbmodel.TrainStation{}).Where("id = ?", station.ID).Update("line_info", station.LineInfo).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &station, nil
}

func UpdateTrainStationLines(mapName, identifier string, lines []initmodel.LineInfo) (*dbmodel.TrainStation, error) {
	var station dbmodel.TrainStation
	station.MapName = strings.TrimSpace(mapName)
	station.Identifier = strings.TrimSpace(identifier)
	tx := database.Connection.Begin()
	if err := station.GetByMapAndIdentifier(tx); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := replaceStationLineRelationsTx(tx, station, lines); err != nil {
		tx.Rollback()
		return nil, err
	}

	infoMap, err := buildStationLineInfoMapTx(tx, []uint{station.ID})
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if value, ok := infoMap[station.ID]; ok {
		station.LineInfo = value
	} else {
		station.LineInfo = "[]"
	}
	if err := tx.Model(&dbmodel.TrainStation{}).Where("id = ?", station.ID).Update("line_info", station.LineInfo).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &station, nil
}

func BuildTrainStationSeedPayload(mapName string, stations []dbmodel.TrainStation, version float64) ([]byte, error) {
	ordered := make([]dbmodel.TrainStation, 0, len(stations))
	ordered = append(ordered, stations...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Identifier < ordered[j].Identifier
	})

	payload := initmodel.TrainStationList{
		Type:    "train_station",
		Name:    mapName,
		Version: version,
		Data:    make([]initmodel.StationData, 0, len(ordered)),
	}

	for _, station := range ordered {
		var lines []initmodel.LineInfo
		if strings.TrimSpace(station.LineInfo) != "" {
			if err := json.Unmarshal([]byte(station.LineInfo), &lines); err != nil {
				return nil, err
			}
		}

		payload.Data = append(payload.Data, initmodel.StationData{
			Label:      station.Label,
			LocalName:  station.StationLocalName,
			Identifier: station.Identifier,
			PhotoXY: initmodel.Coordinate{
				X: station.PhotoX,
				Y: station.PhotoY,
			},
			MapXY: initmodel.Coordinate{
				X: station.MapX,
				Y: station.MapY,
			},
			Line: lines,
		})
	}

	return json.MarshalIndent(payload, "", "    ")
}

func ExportTrainStationSeedJSON(mapName string) (string, error) {
	stations, err := GetAllTrainStationsForAdmin(mapName)
	if err != nil {
		return "", err
	}

	version := 1.0
	var dataRecord dbmodel.DataRecord
	dataRecord.RelatedTable = constant.TrainStation
	dataRecord.RelatedName = mapName
	exists, err := dataRecord.CheckRecordExist(database.Connection)
	if err != nil {
		return "", err
	}
	if exists {
		version = dataRecord.Version
	}

	payload, err := BuildTrainStationSeedPayload(mapName, stations, version)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

type TrainStationImportResult struct {
	MapName          string
	ImportedStations int
	Version          float64
}

func ImportTrainStationSeedJSON(mapName string, raw []byte) (*TrainStationImportResult, error) {
	trimmedMapName := strings.TrimSpace(mapName)
	if trimmedMapName == "" {
		return nil, fmt.Errorf("map_name is required")
	}
	if err := EnsureTrainStationMapExists(trimmedMapName); err != nil {
		return nil, err
	}

	var payload initmodel.TrainStationList
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data) == 0 {
		return &TrainStationImportResult{
			MapName:          trimmedMapName,
			ImportedStations: 0,
			Version:          payload.Version,
		}, nil
	}

	for _, item := range payload.Data {
		lineBytes, err := json.Marshal(item.Line)
		if err != nil {
			return nil, err
		}
		if _, err := UpsertTrainStationByIdentifier(dbmodel.TrainStation{
			MapName:          trimmedMapName,
			Identifier:       strings.TrimSpace(item.Identifier),
			Label:            strings.TrimSpace(item.Label),
			StationLocalName: strings.TrimSpace(item.LocalName),
			PhotoX:           item.PhotoXY.X,
			PhotoY:           item.PhotoXY.Y,
			MapX:             item.MapXY.X,
			MapY:             item.MapXY.Y,
			LineInfo:         string(lineBytes),
		}); err != nil {
			return nil, err
		}
	}

	var dataRecord dbmodel.DataRecord
	dataRecord.RelatedTable = constant.TrainStation
	dataRecord.RelatedName = trimmedMapName
	exists, err := dataRecord.CheckRecordExist(database.Connection)
	if err != nil {
		return nil, err
	}
	if exists {
		dataRecord.Version = payload.Version
		if err := dataRecord.Update(database.Connection); err != nil {
			return nil, err
		}
	}

	return &TrainStationImportResult{
		MapName:          trimmedMapName,
		ImportedStations: len(payload.Data),
		Version:          payload.Version,
	}, nil
}
