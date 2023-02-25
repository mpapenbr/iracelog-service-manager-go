package model

type DbTrack struct {
	ID   int       `json:"id"`
	Data TrackInfo `json:"data"`
}

//nolint:tagliatelle //different structs need to be mapped
type TrackInfo struct {
	ID        int     `json:"trackId"`
	Name      string  `json:"trackDisplayName"`
	ShortName string  `json:"trackDisplayShortName"`
	Config    string  `json:"trackConfigName"`
	Length    float64 `json:"trackLength"`
	Pit       struct {
		Exit       float64 `json:"exit"`
		Entry      float64 `json:"entry"`
		LaneLength float64 `json:"laneLength"`
	} `json:"pit"`
	Sectors []struct {
		SectorNum      int     `json:"sectorNum"`
		SectorStartpct float64 `json:"sectorStartPct"`
	} `json:"sectors"`
}
