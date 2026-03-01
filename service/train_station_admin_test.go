package service

import (
	"encoding/json"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/initdb/initmodel"
	"testing"
)

func TestBuildTrainStationSeedPayloadRoundTrip(t *testing.T) {
	stations := []dbmodel.TrainStation{
		{
			Label:            "Alpha",
			StationLocalName: "阿法",
			Identifier:       "alpha",
			PhotoX:           10,
			PhotoY:           20,
			MapX:             22.1,
			MapY:             114.1,
			LineInfo:         `[{"name":"Line A","localName":"甲線","colour":"#123456","position":1}]`,
			MapName:          "HK_MTR",
		},
		{
			Label:            "Beta",
			StationLocalName: "貝塔",
			Identifier:       "beta",
			PhotoX:           30,
			PhotoY:           40,
			MapX:             22.2,
			MapY:             114.2,
			LineInfo:         `[]`,
			MapName:          "HK_MTR",
		},
	}

	payload, err := BuildTrainStationSeedPayload("HK_MTR", stations, 2)
	if err != nil {
		t.Fatalf("BuildTrainStationSeedPayload returned error: %v", err)
	}

	var parsed initmodel.TrainStationList
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("exported JSON cannot be parsed by seed struct: %v", err)
	}

	if parsed.Type != "train_station" || parsed.Name != "HK_MTR" || parsed.Version != 2 {
		t.Fatalf("unexpected export metadata: %+v", parsed)
	}
	if len(parsed.Data) != 2 {
		t.Fatalf("expected 2 station entries, got %d", len(parsed.Data))
	}
	if parsed.Data[0].Identifier != "alpha" {
		t.Fatalf("expected deterministic identifier sorting, got first=%s", parsed.Data[0].Identifier)
	}
	if len(parsed.Data[0].Line) != 1 || parsed.Data[0].Line[0].Name != "Line A" {
		t.Fatalf("line data not round-tripped correctly: %+v", parsed.Data[0].Line)
	}
}
