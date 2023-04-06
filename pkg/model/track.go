package model

type DbTrack struct {
	ID   int       `json:"id"`
	Data TrackInfo `json:"data"`
}

//nolint:tagliatelle //different structs need to be mapped
type TrackInfo struct {
	ID        int      `json:"trackId"`
	Name      string   `json:"trackDisplayName"`
	ShortName string   `json:"trackDisplayShortName"`
	Config    string   `json:"trackConfigName"`
	Length    float64  `json:"trackLength"`
	Pit       *PitInfo `json:"pit,omitempty"`
	Sectors   []Sector `json:"sectors"`
}

type Sector struct {
	SectorNum      int
	SectorStartPct float64
}
type PitInfo struct {
	Exit       float64 `json:"exit"`
	Entry      float64 `json:"entry"`
	LaneLength float64 `json:"laneLength"`
}
