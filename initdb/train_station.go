package initdb

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/initdb/initmodel"
	"os"
)

func SeedAllTrainStation() error {
	if err := SeedTrainStationData(constant.HKMTRJson, constant.HKMTR); err != nil {
		return err
	}
	if err := SeedTrainStationData(constant.TORTCCJson, constant.TORTCC); err != nil {
		return err
	}
	return nil
}

func SeedTrainStationData(jsonName, identifier string) error {
	trainData, err := ReadTrainJson(jsonName)
	if err != nil {
		return err
	}

	dataRecord, shouldUpdate, err := CheckShouldUpdate(constant.TrainStation, identifier, trainData.Version)
	if err != nil {
		return err
	}

	if shouldUpdate {
		log.Println("Detected version different for " + identifier + " , updating...")
		transaction := database.Connection.Begin()

		dataRecord.Version = trainData.Version
		if err := dataRecord.Update(transaction); err != nil {
			transaction.Rollback()
			return err
		}

		for _, stationData := range trainData.Data {
			var trainStation dbmodel.TrainStation

			trainStation.Label = stationData.Label
			trainStation.StationLocalName = stationData.LocalName
			trainStation.Identifier = stationData.Identifier
			trainStation.PhotoX = stationData.PhotoXY.X
			trainStation.PhotoY = stationData.PhotoXY.Y
			trainStation.MapX = stationData.MapXY.X
			trainStation.MapY = stationData.MapXY.Y
			bytes, err := json.Marshal(stationData.Line)
			if err != nil {
				transaction.Rollback()
				return err
			}
			trainStation.LineInfo = string(bytes)
			trainStation.MapName = identifier

			if err := trainStation.UpdateByMapAndIdentifier(transaction); err != nil {
				transaction.Rollback()
				return err
			}
		}

		transaction.Commit()
	}

	return nil
}

// func SeedHKMTRStation() error {
// 	trainData, err := ReadTrainJson("hkmtr.json")
// 	if err != nil {
// 		return err
// 	}

// 	dataRecord, shouldUpdate, err := CheckShouldUpdate(constant.TrainStation, constant.HKMTR, trainData.Version)
// 	if err != nil {
// 		return err
// 	}

// 	if shouldUpdate {
// 		log.Println("Detected version different for HK_MTR, updating...")
// 		transaction := database.Connection.Begin()

// 		dataRecord.Version = trainData.Version
// 		if err := dataRecord.Update(transaction); err != nil {
// 			transaction.Rollback()
// 			return err
// 		}

// 		for _, stationData := range trainData.Data {
// 			var trainStation dbmodel.TrainStation

// 			trainStation.Label = stationData.Label
// 			trainStation.StationLocalName = stationData.LocalName
// 			trainStation.Identifier = stationData.Identifier
// 			trainStation.PhotoX = stationData.PhotoXY.X
// 			trainStation.PhotoY = stationData.PhotoXY.Y
// 			trainStation.MapX = stationData.MapXY.X
// 			trainStation.MapY = stationData.MapXY.Y
// 			bytes, err := json.Marshal(stationData.Line)
// 			if err != nil {
// 				transaction.Rollback()
// 				return err
// 			}
// 			trainStation.LineInfo = string(bytes)
// 			trainStation.MapName = constant.HKMTR

// 			if err := trainStation.UpdateByMapAndIdentifier(transaction); err != nil {
// 				transaction.Rollback()
// 				return err
// 			}
// 		}

// 		transaction.Commit()
// 	}

// 	return nil
// }

func CheckShouldUpdate(table, name string, version float64) (*dbmodel.DataRecord, bool, error) {
	shouldUpdate := false

	var dataRecord dbmodel.DataRecord
	dataRecord.RelatedTable = table
	dataRecord.RelatedName = name
	exist, err := dataRecord.CheckRecordExist(database.Connection)
	if err != nil {
		return nil, false, err
	}

	if exist {
		if dataRecord.Version != version {
			shouldUpdate = true
		}
	} else {
		dataRecord.Version = 0
		if err := dataRecord.Create(database.Connection); err != nil {
			return nil, false, err
		}
		shouldUpdate = true
	}

	return &dataRecord, shouldUpdate, nil
}

func ReadTrainJson(path string) (*initmodel.TrainStationList, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	prefix := pwd + "/json/"
	jsonFile, err := os.Open(prefix + path)
	if err != nil {
		return nil, err
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var data initmodel.TrainStationList

	json.Unmarshal(byteValue, &data)

	defer jsonFile.Close()

	return &data, nil
}
