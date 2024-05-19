package model

type DbTrack struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	ShortName     string   `json:"shortName"`
	Config        string   `json:"config"`
	TrackLength   float64  `json:"trackLength"`
	PitExit       float64  `json:"pitExit"`
	PitEntry      float64  `json:"pitEntry"`
	PitLaneLength float64  `json:"pitLaneLength"`
	Sectors       []Sector `json:"sectors"`
}
type Sector struct {
	SectorNum      int     `json:"sectorNum"`
	SectorStartPct float64 `json:"sectorStartPct"`
}
