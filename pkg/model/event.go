package model

import "time"

//nolint:tagliatelle // client compatibility
type DbEvent struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"eventKey"`
	Description string    `json:"description"`
	RecordStamp time.Time `json:"recordDate"`
	Data        EventData `json:"data"`
}
type DbEventExtra struct {
	ID      int       `json:"id"`
	EventID int       `json:"eventId"`
	Data    ExtraInfo `json:"data"`
}

type EventData struct {
	Info       EventDataInfo `json:"info"`
	Manifests  Manifests     `json:"manifests"`
	ReplayInfo ReplayInfo    `json:"replayInfo"`
}

type EventDataInfo struct {
	TrackId               int      `json:"trackId"`
	TrackDisplayName      string   `json:"trackDisplayName"`
	TrackDisplayShortName string   `json:"trackDisplayShortName"`
	TrackConfigName       string   `json:"trackConfigName"`
	TrackLength           float64  `json:"trackLength"`
	TrackPitSpeed         float64  `json:"trackPitSpeed"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	EventTime             string   `json:"eventTime"`
	RaceloggerVersion     string   `json:"raceloggerVersion"`
	TeamRacing            int      `json:"teamRacing"` // 0: false
	MultiClass            bool     `json:"multiClass"`
	NumCarTypes           int      `json:"numCarTypes"`
	NumCarClasses         int      `json:"numCarClasses"`
	IrSessionId           int      `json:"irSessionId"`
	Sectors               []Sector `json:"sectors"`
	Sessions              []struct {
		Num  int    `json:"num"`
		Name string `json:"name"`
	} `json:"sessions"`
}

type Manifests struct {
	Car     []string `json:"car"`
	Pit     []string `json:"pit"`
	Message []string `json:"message"`
	Session []string `json:"session"`
}
type ReplayInfo struct {
	MinTimestamp   float64 `json:"minTimestamp"`
	MinSessionTime float64 `json:"minSessionTime"`
	MaxSessionTime float64 `json:"maxSessionTime"`
}

type ExtraInfo struct {
	Track TrackInfo `json:"track"`
}
