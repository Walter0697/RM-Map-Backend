package service

import (
	"encoding/json"
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
	return stations, nil
}

func UpsertTrainStationByIdentifier(station dbmodel.TrainStation) (*dbmodel.TrainStation, error) {
	station.Identifier = strings.TrimSpace(station.Identifier)
	station.MapName = strings.TrimSpace(station.MapName)
	if station.Identifier == "" || station.MapName == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var existing dbmodel.TrainStation
	err := database.Connection.Where("identifier = ? AND map_name = ?", station.Identifier, station.MapName).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if createErr := station.Create(database.Connection); createErr != nil {
				return nil, createErr
			}
			return &station, nil
		}
		return nil, err
	}

	station.ID = existing.ID
	if saveErr := station.Update(database.Connection); saveErr != nil {
		return nil, saveErr
	}
	return &station, nil
}

func UpdateTrainStationLines(mapName, identifier string, lines []initmodel.LineInfo) (*dbmodel.TrainStation, error) {
	var station dbmodel.TrainStation
	station.MapName = strings.TrimSpace(mapName)
	station.Identifier = strings.TrimSpace(identifier)
	if err := station.GetByMapAndIdentifier(database.Connection); err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(lines)
	if err != nil {
		return nil, err
	}
	station.LineInfo = string(bytes)
	if err := station.Update(database.Connection); err != nil {
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
