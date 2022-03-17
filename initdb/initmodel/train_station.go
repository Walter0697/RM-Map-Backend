package initmodel

type Coordinate struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type LineInfo struct {
	Name      string `json:"name"`
	LocalName string `json:"localName"`
	Position  uint   `json:"position"`
	Colour    string `json:"colour"`
}

type StationData struct {
	Label      string     `json:"label"`
	LocalName  string     `json:"localName"`
	Identifier string     `json:"identifier"`
	PhotoXY    Coordinate `json:"photo"`
	MapXY      Coordinate `json:"map"`
	Line       []LineInfo `json:"line"`
}

type TrainStationList struct {
	Type    string        `json:"type"`
	Name    string        `json:"name"`
	Version float64       `json:"version"`
	Data    []StationData `json:"data"`
}
